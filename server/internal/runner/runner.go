// Package runner executes extracted upy functionality in an isolated child process.
// Credentials arrive over stdin, never command arguments or public task records.
package runner

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
	"pier/internal/engine/backup"
	"pier/internal/engine/bundle"
	"pier/internal/engine/cleaner"
	ec "pier/internal/engine/config"
	"pier/internal/engine/crypto"
	"pier/internal/engine/deploy"
	"pier/internal/engine/github"
	"pier/internal/engine/i18n"
	"pier/internal/engine/log"
	"pier/internal/engine/notify"
	"pier/internal/engine/platform"
	"pier/internal/store"
	"time"
)

type Input struct {
	Project store.Project `json:"project"`
	Root    string        `json:"root"`
	Kind    string        `json:"kind"`
	Target  string        `json:"target"`
	Upload  string        `json:"upload"`
	Force   bool          `json:"force"`
}

func Run(ctx context.Context, in Input) error {
	if os.Getenv("PIER_CONTAINER") == "1" && in.Project.Type == "system" && (in.Kind == "deploy" || in.Kind == "rollback") {
		return errors.New("容器版不支持管理宿主机 systemd 服务，请在宿主机运行 Pier 二进制版本")
	}
	_ = i18n.SetLanguage("zh-CN")
	platform.SetDefaultUmask()
	private, e := os.MkdirTemp("", "pier-worker-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(private)
	user := ec.UserConfig{GitHub: ec.UserGitHub{Token: in.Project.GitHubToken}, Language: "zh-CN"}
	b, e := yaml.Marshal(user)
	if e != nil {
		return e
	}
	conf := filepath.Join(private, "config.yaml")
	if e = os.WriteFile(conf, b, 0600); e != nil {
		return e
	}
	_ = os.Setenv("UPY_CONFIG", conf)
	// Do not inherit another application's default GitHub credential.
	_ = os.Unsetenv("DEPLOY_GITHUB_TOKEN")
	_ = os.Unsetenv("GITHUB_TOKEN")
	log.RegisterSecret(in.Project.GitHubToken)
	cfg, e := ec.ReadProjectConfig(in.Root)
	if e != nil {
		return e
	}
	log.RegisterSecret(cfg.CryptoPassword())
	switch in.Kind {
	case "releases":
		if in.Project.Repository == "" {
			return errors.New("尚未配置 GitHub 仓库")
		}
		releases, e := github.ListReleases(in.Project.Repository)
		if e != nil {
			return e
		}
		return json.NewEncoder(os.Stdout).Encode(releases)
	case "deploy":
		path := in.Upload
		if path == "" {
			if in.Project.Repository == "" {
				return errors.New("请配置 GitHub 仓库，或上传本地部署包")
			}
			log.Step(fmt.Sprintf("正在查询 GitHub Release：%s，目标 %s", in.Project.Repository, in.Target))
			var rel *github.GHRelease
			if in.Target == "" || in.Target == "latest" {
				rel, e = github.GetLatest(in.Project.Repository)
			} else {
				rel, e = github.GetByTag(in.Project.Repository, in.Target)
			}
			if e != nil {
				return e
			}
			log.Ok("已获取 Release：" + rel.Tag)
			asset, e := github.FindAsset(rel, in.Project.Asset, in.Project.ID, filepath.Base(in.Project.Repository))
			if e != nil {
				return e
			}
			dir := filepath.Join(in.Root, "bundles")
			if e = os.MkdirAll(dir, 0755); e != nil {
				return e
			}
			// Never trust asset names or tags as filesystem paths.
			f, e := os.CreateTemp(dir, "release-*.zip")
			if e != nil {
				return e
			}
			path = f.Name()
			f.Close()
			log.Step(fmt.Sprintf("开始下载 %s（%s）", asset.Name, log.HumanSize(asset.Size)))
			if e = github.DownloadAsset(in.Project.Repository, asset, path); e != nil {
				os.Remove(path)
				return e
			}
			defer func() {
				if e != nil {
					os.Remove(path)
				}
			}()
			log.Ok("下载完成，正在校验部署包名称、类型和版本")
			if e = validate(path, in.Project, rel.Tag); e != nil {
				return e
			}
		} else {
			log.Step("正在校验本地部署包")
			if e = validate(path, in.Project, ""); e != nil {
				return e
			}
		}
		log.Ok("部署包校验通过，开始安装和激活版本")
		_, e = deploy.DeployBundle(path, deploy.Options{Root: in.Root, Force: in.Force})
		return e
	case "rollback":
		if in.Target == "" || filepath.Base(in.Target) != in.Target || strings.ContainsAny(in.Target, "/\\") || in.Target == "." || in.Target == ".." {
			return errors.New("回退版本无效")
		}
		source := filepath.Join(in.Root, "releases", in.Target)
		m, e := bundle.ReadManifest(source)
		if e != nil {
			return e
		}
		if m.Version != in.Target || m.Type != in.Project.Type {
			return errors.New("版本目录与清单不匹配")
		}
		f, e := os.CreateTemp(private, "rollback-*.zip")
		if e != nil {
			return e
		}
		path := f.Name()
		f.Close()
		if e = archive(source, path); e != nil {
			return e
		}
		_, e = deploy.DeployBundle(path, deploy.Options{Root: in.Root, Force: true})
		return e
	case "backup":
		if runtime.GOOS == "windows" {
			return errors.New("备份脚本请在 Linux / macOS 服务器上执行")
		}
		started := time.Now()
		res, e := backup.Run(ctx, backup.Options{Root: in.Root, Stdout: os.Stdout, Stderr: os.Stderr})
		event := notify.Event{Kind: notify.KindBackupSuccess, Project: in.Project.Name, Root: in.Root, Elapsed: time.Since(started).Round(time.Second).String()}
		if e != nil {
			event.Kind = notify.KindBackupFailure
			event.Error = e.Error()
		}
		notify.Send(event)
		if e != nil {
			return e
		}
		fmt.Printf("备份完成：新增 %d 个归档，%d 字节\n", len(res.CreatedFiles), res.TotalBytes)
		return nil
	case "clean":
		if in.Target == "tmp" {
			n, _, e := cleaner.CleanTmpDeployDirs(in.Root)
			fmt.Printf("清理残留临时目录：%d 字节\n", n)
			return e
		}
		if in.Target == "bundles" {
			n, _, e := cleaner.CleanBundles(in.Root)
			fmt.Printf("清理 Bundle 缓存：%d 字节\n", n)
			return e
		}
		if in.Target == "releases" {
			n, _, e := cleaner.CleanOldReleases(in.Root)
			fmt.Printf("清理旧版本：%d 字节\n", n)
			return e
		}
		return errors.New("请选择 bundles、releases 或 tmp")
	default:
		return errors.New("不支持的任务类型")
	}
}

func validate(path string, p store.Project, tag string) error {
	private, e := os.MkdirTemp("", "pier-validate-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(private)
	enc, e := crypto.IsEncryptedFile(path)
	if e != nil {
		return e
	}
	if enc {
		var cfg ec.ProjectConfig
		if e = yaml.Unmarshal([]byte(p.ConfigYAML), &cfg); e != nil {
			return e
		}
		password := cfg.CryptoPassword()
		if password == "" {
			return errors.New("部署包已加密，请配置 crypto.password")
		}
		plain := filepath.Join(private, "plain.zip")
		if e = crypto.DecryptFile(path, plain, password); e != nil {
			return e
		}
		path = plain
	}
	extracted := filepath.Join(private, "unpacked")
	if e = bundle.SafeExtract(path, extracted); e != nil {
		return e
	}
	m, e := bundle.ReadManifest(extracted)
	if e != nil {
		return e
	}
	if m.Name != p.ID {
		return fmt.Errorf("部署包名称 %q 与项目标识 %q 不一致", m.Name, p.ID)
	}
	if m.Type != p.Type {
		return fmt.Errorf("部署包类型 %q 与项目类型 %q 不一致", m.Type, p.Type)
	}
	if tag != "" && strings.TrimPrefix(m.Version, "v") != strings.TrimPrefix(tag, "v") {
		return errors.New("部署包版本与 Release 标签不一致")
	}
	if m.Type == "system" && runtime.GOOS != "linux" {
		return errors.New("systemd 服务只能部署到 Linux 主机")
	}
	return nil
}
func archive(root, target string) error {
	f, e := os.Create(target)
	if e != nil {
		return e
	}
	z := zip.NewWriter(f)
	e = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("版本目录含符号链接，拒绝打包回退")
		}
		if info.IsDir() {
			return nil
		}
		rel, e := filepath.Rel(root, path)
		if e != nil {
			return e
		}
		h, e := zip.FileInfoHeader(info)
		if e != nil {
			return e
		}
		h.Name = filepath.ToSlash(rel)
		h.Method = zip.Deflate
		out, e := z.CreateHeader(h)
		if e != nil {
			return e
		}
		in, e := os.Open(path)
		if e != nil {
			return e
		}
		defer in.Close()
		_, e = io.Copy(out, in)
		return e
	})
	ze := z.Close()
	fe := f.Close()
	return errors.Join(e, ze, fe)
}
