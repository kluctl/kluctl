// This is copied from https://github.com/sosedoff/gitkit and simplified
package http_server

import (
	"compress/gzip"
	"fmt"
	process2 "github.com/codablock/kluctl/pkg/utils/process"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strings"
)

type service struct {
	method  string
	suffix  string
	handler func(string, http.ResponseWriter, *Request)
	rpc     string
}

type Server struct {
	baseDir string
	services []service
}

type Request struct {
	*http.Request
	RepoName string
	RepoPath string
}

func New(baseDir string) *Server {
	s := Server{baseDir: baseDir}
	s.services = []service{
		service{"GET", "/info/refs", s.getInfoRefs, ""},
		service{"POST", "/git-upload-pack", s.postRPC, "git-upload-pack"},
		service{"POST", "/git-receive-pack", s.postRPC, "git-receive-pack"},
	}

	return &s
}

// findService returns a matching git subservice and parsed repository name
func (s *Server) findService(req *http.Request) (*service, string) {
	for _, svc := range s.services {
		if svc.method == req.Method && strings.HasSuffix(req.URL.Path, svc.suffix) {
			path := strings.Replace(req.URL.Path, svc.suffix, "", 1)
			return &svc, path
		}
	}
	return nil, ""
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Info("request", r.Method+" "+r.Host+r.URL.String())

	// Find the git subservice to handle the request
	svc, repoUrlPath := s.findService(r)
	if svc == nil {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Determine namespace and repo name from request path
	repoNamespace, repoName := getNamespaceAndRepo(repoUrlPath)
	if repoName == "" {
		log.Error("auth", fmt.Errorf("no repo name provided"))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	req := &Request{
		Request:  r,
		RepoName: path.Join(repoNamespace, repoName),
		RepoPath: path.Join(s.baseDir, repoNamespace, repoName),
	}

	if !repoExists(req.RepoPath) {
		log.Error("repo-init", fmt.Errorf("%s does not exist", req.RepoPath))
		http.NotFound(w, r)
		return
	}

	svc.handler(svc.rpc, w, req)
}

func (s *Server) getInfoRefs(_ string, w http.ResponseWriter, r *Request) {
	context := "get-info-refs"
	rpc := r.URL.Query().Get("service")

	if !(rpc == "git-upload-pack" || rpc == "git-receive-pack") {
		http.Error(w, "Not Found", 404)
		return
	}

	cmd, pipe := gitCommand("git", subCommand(rpc), "--stateless-rpc", "--advertise-refs", r.RepoPath)
	if err := cmd.Start(); err != nil {
		fail500(w, context, err)
		return
	}
	defer cleanUpProcessGroup(cmd)

	w.Header().Add("Content-Type", fmt.Sprintf("application/x-%s-advertisement", rpc))
	w.Header().Add("Cache-Control", "no-cache")
	w.WriteHeader(200)

	if err := packLine(w, fmt.Sprintf("# service=%s\n", rpc)); err != nil {
		log.Error(context, err)
		return
	}

	if err := packFlush(w); err != nil {
		log.Error(context, err)
		return
	}

	if _, err := io.Copy(w, pipe); err != nil {
		log.Error(context, err)
		return
	}

	if err := cmd.Wait(); err != nil {
		log.Error(context, err)
		return
	}
}

func (s *Server) postRPC(rpc string, w http.ResponseWriter, r *Request) {
	context := "post-rpc"
	body := r.Body

	if r.Header.Get("Content-Encoding") == "gzip" {
		var err error
		body, err = gzip.NewReader(r.Body)
		if err != nil {
			fail500(w, context, err)
			return
		}
	}

	cmd, pipe := gitCommand("git", subCommand(rpc), "--stateless-rpc", r.RepoPath)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		fail500(w, context, err)
		return
	}
	defer stdin.Close()

	if err := cmd.Start(); err != nil {
		fail500(w, context, err)
		return
	}
	defer cleanUpProcessGroup(cmd)

	if _, err := io.Copy(stdin, body); err != nil {
		fail500(w, context, err)
		return
	}

	w.Header().Add("Content-Type", fmt.Sprintf("application/x-%s-result", rpc))
	w.Header().Add("Cache-Control", "no-cache")
	w.WriteHeader(200)

	if _, err := io.Copy(newWriteFlusher(w), pipe); err != nil {
		log.Error(context, err)
		return
	}
	if err := cmd.Wait(); err != nil {
		log.Error(context, err)
		return
	}
}

func repoExists(p string) bool {
	_, err := os.Stat(path.Join(p, "objects"))
	return err == nil
}

func gitCommand(name string, args ...string) (*exec.Cmd, io.Reader) {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = process2.ProcAttrWithProcessGroup
	cmd.Env = os.Environ()

	r, _ := cmd.StdoutPipe()
	cmd.Stderr = cmd.Stdout

	return cmd, r
}