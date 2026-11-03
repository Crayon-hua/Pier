package task

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"pier/internal/project"
	"pier/internal/runner"
	"pier/internal/store"
)

type Service struct {
	Store    *store.Store
	Projects *project.Service
	DataDir  string
	mu       sync.Mutex
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
	closing  bool
}

func New(s *store.Store, p *project.Service, dir string) *Service {
	ctx, cancel := context.WithCancel(context.Background())
	return &Service{Store: s, Projects: p, DataDir: dir, ctx: ctx, cancel: cancel}
}
func ID() string {
	b := make([]byte, 12)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func (s *Service) LogPath(id string) string { return filepath.Join(s.DataDir, "logs", id+".log") }
func (s *Service) Start(id, kind, target, upload string, force bool) (store.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closing {
		return store.Task{}, errors.New("服务正在关闭")
	}
	p, e := s.Projects.Get(id)
	if e != nil {
		return store.Task{}, e
	}
	t := store.Task{ID: ID(), ProjectID: id, ProjectName: p.Name, Kind: kind, Target: target, Force: force, Status: "queued", CreatedAt: time.Now()}
	if e = os.MkdirAll(filepath.Join(s.DataDir, "logs"), 0700); e != nil {
		return t, e
	}
	if e = s.Store.Update(func(d *store.Data) error {
		if _, ok := d.Projects[id]; !ok {
			return errors.New("项目不存在")
		}
		for _, v := range d.Tasks {
			if v.ProjectID == id && (v.Status == "running" || v.Status == "queued") {
				return errors.New("项目已有任务正在执行")
			}
		}
		p = d.Projects[id]
		d.Tasks[t.ID] = t
		return nil
	}); e != nil {
		return t, e
	}
	s.wg.Add(1)
	go func() { defer s.wg.Done(); s.execute(t, p, upload) }()
	return t, nil
}
func (s *Service) execute(t store.Task, p store.Project, upload string) {
	if upload != "" {
		defer os.Remove(upload)
	}
	now := time.Now()
	t.Status = "running"
	t.StartedAt = &now
	if e := s.Store.Update(func(d *store.Data) error { d.Tasks[t.ID] = t; return nil }); e != nil {
		s.finish(t, e)
		return
	}
	f, e := os.OpenFile(s.LogPath(t.ID), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if e != nil {
		s.finish(t, e)
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "[%s] %s · %s\n", now.Format(time.RFC3339), p.Name, t.Kind)
	if e = s.Projects.WriteConfig(p); e == nil {
		in := runner.Input{Project: p, Root: s.Projects.Root(p.ID), Kind: t.Kind, Target: t.Target, Upload: upload, Force: t.Force}
		// Every task gets a child process: upy's package-level logger/config cannot
		// accidentally cross-contaminate concurrent projects.
		ctx, cancel := context.WithTimeout(s.ctx, 45*time.Minute)
		defer cancel()
		done := make(chan struct{})
		stopped := make(chan struct{})
		go func() {
			defer close(stopped)
			ticker := time.NewTicker(15 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					fmt.Fprintf(f, "[%s] 任务仍在运行，累计耗时 %s；若当前阶段无新增进度，请检查服务器网络或外部命令响应。\n", time.Now().Format(time.RFC3339), time.Since(now).Round(time.Second))
				}
			}
		}()
		e = s.command(ctx, in, f)
		close(done)
		<-stopped
	}
	if e != nil {
		fmt.Fprintf(f, "\n任务未完成：%v\n", e)
	} else {
		fmt.Fprintln(f, "\n任务完成")
	}
	s.finish(t, e)
}
func (s *Service) command(ctx context.Context, in runner.Input, out io.Writer) error {
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	payload, e := json.Marshal(in)
	if e != nil {
		return e
	}
	cmd := exec.CommandContext(ctx, exe, "worker")
	cmd.Stdin = bytes.NewReader(payload)
	cmd.Stdout = out
	cmd.Stderr = out
	err := cmd.Run()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}
func (s *Service) finish(t store.Task, err error) {
	now := time.Now()
	t.FinishedAt = &now
	t.Status = "succeeded"
	if err != nil {
		t.Status = "failed"
		t.Error = "执行失败，请查看任务日志"
		if s.ctx.Err() != nil || errors.Is(err, context.DeadlineExceeded) {
			t.Status = "interrupted"
			t.Error = "任务执行被中断，最终结果待确认"
		}
	}
	if e := s.Store.Update(func(d *store.Data) error { d.Tasks[t.ID] = t; return nil }); e != nil {
		fmt.Fprintln(os.Stderr, "persist task:", e)
	}
}
func (s *Service) Releases(ctx context.Context, p store.Project) (json.RawMessage, error) {
	// The release query must not write a project's deployment configuration while
	// another task is running. It uses a temporary root instead.
	root, e := os.MkdirTemp("", "pier-query-")
	if e != nil {
		return nil, e
	}
	defer os.RemoveAll(root)
	b, e := project.Config(p)
	if e != nil {
		return nil, e
	}
	if e = os.WriteFile(filepath.Join(root, "deploy.yaml"), b, 0600); e != nil {
		return nil, e
	}
	exe, e := os.Executable()
	if e != nil {
		return nil, e
	}
	payload, _ := json.Marshal(runner.Input{Project: p, Root: root, Kind: "releases"})
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, "worker")
	cmd.Stdin = bytes.NewReader(payload)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if e = cmd.Run(); e != nil {
		return nil, errors.New("无法获取 Release，请检查仓库、Token 和服务器网络")
	}
	if !json.Valid(stdout.Bytes()) {
		return nil, errors.New("Release 响应无效")
	}
	return json.RawMessage(stdout.Bytes()), nil
}

// Shutdown lets running deployments finish rather than abruptly killing a service
// between deactivation and activation. A process manager must allow sufficient time.
func (s *Service) Shutdown() { s.mu.Lock(); s.closing = true; s.mu.Unlock(); s.wg.Wait(); s.cancel() }
