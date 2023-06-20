package main

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"filippo.io/age"
	"flag"
	"fmt"
	"github.com/ipfs/boxo/coreiface/options"
	"github.com/ipfs/boxo/files"
	log "github.com/sirupsen/logrus"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ipfs/kubo/client/rpc"
)

var modeFlag string
var ipnsKey string
var ipnsName string
var ipfsId string
var staticIpnsName string
var prNumber int
var ageKeyFile string
var agePubKey string
var repoName string

func ParseFlags() error {
	flag.StringVar(&modeFlag, "mode", "", "Mode")
	flag.StringVar(&ipnsKey, "ipns-key", "", "IPNS key name")
	flag.StringVar(&ipnsName, "ipns-name", "", "IPNS name")
	flag.StringVar(&staticIpnsName, "static-ipns-name", "", "Static Webui IPNS name")
	flag.IntVar(&prNumber, "pr-number", 0, "PR number")
	flag.StringVar(&ipfsId, "ipfs-id", "", "IPFS id")
	flag.StringVar(&ageKeyFile, "age-key-file", "", "AGE key file")
	flag.StringVar(&agePubKey, "age-pub-key", "", "AGE pubkey")
	flag.StringVar(&repoName, "repo-name", "", "Repo name")
	flag.Parse()

	return nil
}

func main() {
	err := ParseFlags()
	if err != nil {
		panic(err)
	}

	// "Connect" to local node
	node, err := rpc.NewLocalApi()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}

	switch modeFlag {
	case "publish-ipns":
		err = doPublishIpns(node)
	case "resolve-ipns":
		err = doResolve(node)
	case "send-info":
		err = doSend(node)
	case "receive-info":
		err = doReceive(node)
	default:
		err = fmt.Errorf("unknown mode %s", modeFlag)
	}

	if err != nil {
		log.Error(err)
		log.Exit(1)
	} else {
		log.Exit(0)
	}
}

type ipnsInfo struct {
	PrNumber int    `json:"prNumber"`
	IpfsId   string `json:"ipfsId"`
}

type workflowInfo struct {
	PrNumber       int    `json:"prNumber"`
	IpfsId         string `json:"ipfsId"`
	StaticIpnsName string `json:"staticIpnsName"`
	GithubToken    string `json:"githubToken"`
}

func doPublishIpns(node *rpc.HttpApi) error {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	selfKey, err := node.Key().Self(ctx)
	if err != nil {
		return err
	}

	info := ipnsInfo{
		PrNumber: prNumber,
		IpfsId:   selfKey.ID().String(),
	}
	b, err := json.Marshal(&info)
	if err != nil {
		return err
	}
	log.Info("publishing: ", string(b))

	f := files.NewBytesFile(b)

	pth, err := node.Unixfs().Add(ctx, f)
	if err != nil {
		return err
	}

	log.Info("path: ", pth.String())

	ipnsEntry, err := node.Name().Publish(ctx, pth, options.Name.Key(ipnsKey), options.Name.TTL(10*time.Second))
	if err != nil {
		return err
	}

	log.Info("published as ", ipnsEntry.Name())

	return nil
}

func doResolve(node *rpc.HttpApi) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Info("Resolving: ", ipnsName)

	resolved, err := node.Name().Resolve(ctx, ipnsName, options.Name.Cache(false))
	if err != nil {
		return err
	}

	log.Info("Resolved to: ", resolved.String())

	nd, err := node.Unixfs().Get(ctx, resolved)
	if err != nil {
		return err
	}
	defer nd.Close()

	f, ok := nd.(files.File)
	if !ok {
		return fmt.Errorf("%s is not a file", resolved.String())
	}

	b, err := io.ReadAll(f)
	if err != nil {
		return err
	}

	var info ipnsInfo
	err = json.Unmarshal(b, &info)
	if err != nil {
		return err
	}

	log.Info("IPNS Info: ", string(b))

	if info.PrNumber != prNumber {
		return fmt.Errorf("IPNS entry not up-to-date")
	}

	_, err = os.Stdout.WriteString(info.IpfsId + "\n")
	if err != nil {
		return err
	}
	return nil
}

func doSend(node *rpc.HttpApi) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	selfKey, err := node.Key().Self(ctx)
	if err != nil {
		return err
	}

	info := workflowInfo{
		PrNumber:       prNumber,
		GithubToken:    os.Getenv("GITHUB_TOKEN"),
		IpfsId:         selfKey.ID().String(),
		StaticIpnsName: staticIpnsName,
	}

	b, err := json.Marshal(&info)
	if err != nil {
		return err
	}

	ageRecipient, err := age.ParseX25519Recipient(agePubKey)
	if err != nil {
		return err
	}

	w := bytes.NewBuffer(nil)
	e, err := age.Encrypt(w, ageRecipient)
	if err != nil {
		return err
	}
	_, err = e.Write(b)
	if err != nil {
		return err
	}
	err = e.Close()
	if err != nil {
		return err
	}
	b = w.Bytes()

	log.Info("Sending info...")

	err = p2pSendFile(ctx, node, ipfsId, b)
	if err != nil {
		return err
	}

	log.Info("Done sending info.")

	return nil

}

func doReceive(node *rpc.HttpApi) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	log.Info("Receiving info...")

	b, err := p2pReceiveFile(ctx, node)
	if err != nil {
		return err
	}

	log.Info("Done receiving info.")

	idsBytes, err := os.ReadFile(ageKeyFile)
	if err != nil {
		return err
	}
	ageIds, err := age.ParseIdentities(bytes.NewReader(idsBytes))
	if err != nil {
		return err
	}
	d, err := age.Decrypt(bytes.NewReader(b), ageIds...)
	if err != nil {
		return err
	}

	w := bytes.NewBuffer(nil)
	_, err = io.Copy(w, d)
	if err != nil {
		return err
	}
	b = w.Bytes()

	var info workflowInfo
	err = json.Unmarshal(b, &info)
	if err != nil {
		return err
	}

	if info.PrNumber != prNumber {
		return fmt.Errorf("%d is not the expected (%d) PR number", info.PrNumber, prNumber)
	}

	log.Info("Checking Github token...")

	err = checkGithubToken(ctx, info.GithubToken)
	if err != nil {
		return err
	}

	log.Info("Done checking Github token...")

	info.GithubToken = ""

	b, err = json.Marshal(&info)
	if err != nil {
		return err
	}
	_, _ = os.Stdout.WriteString(string(b) + "\n")

	return nil
}

func doGithubRequest(ctx context.Context, method string, url string, body string, token string) ([]byte, error) {
	log.Info("request: ", method, url)

	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		log.Error("NewRequest failed: ", err)
		return nil, err
	}
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("token %s", token))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Error("Request failed: ", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Error(fmt.Sprintf("Request failed: %d - %v", resp.StatusCode, resp.Status))
		return nil, fmt.Errorf("http error: %s", resp.Status)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error("Failed to read body: ", err)
		return nil, err
	}

	return b, nil
}

func checkGithubToken(ctx context.Context, token string) error {
	body := fmt.Sprintf(`{"query": "query UserCurrent{viewer{login}}"}`)
	b, err := doGithubRequest(ctx, "POST", "https://api.github.com/graphql", body, token)
	if err != nil {
		return err
	}
	log.Info("body=", string(b))

	var r struct {
		Data struct {
			Viewer struct {
				Login string `json:"login"`
			} `json:"viewer"`
		} `json:"data"`
	}
	err = json.Unmarshal(b, &r)
	if err != nil {
		log.Error("Unmarshal failed: ", err)
		return err
	}
	if r.Data.Viewer.Login != "github-actions[bot]" {
		log.Error("unexpected response from github")
		return fmt.Errorf("unexpected response from github")
	}

	log.Info("Querying repositories...")

	b, err = doGithubRequest(ctx, "GET", "https://api.github.com/installation/repositories", "", token)
	if err != nil {
		return err
	}
	log.Info("body=", string(b))

	var r2 struct {
		Repositories []struct {
			FullName string `json:"full_name"`
		} `json:"repositories"`
	}
	err = json.Unmarshal(b, &r2)
	if err != nil {
		return err
	}
	if len(r2.Repositories) != 1 {
		return fmt.Errorf("unexpected repositories count %d", len(r2.Repositories))
	}
	if r2.Repositories[0].FullName != repoName {
		return fmt.Errorf("%s is not the expected repo name", r2.Repositories[0].FullName)
	}

	return nil
}

func p2pSendFile(ctx context.Context, node *rpc.HttpApi, ipfsId string, data []byte) error {
	// close the old one
	_, _ = node.Request("p2p/close").
		Option("protocol", "/x/kluctl-preview-info").
		Option("listen-address", "/ip4/127.0.0.1/tcp/10001").
		Send(ctx)

	_, err := node.Request("p2p/forward", "/x/kluctl-preview-info", "/ip4/127.0.0.1/tcp/10001", fmt.Sprintf("/p2p/%s", ipfsId)).
		Send(ctx)
	if err != nil {
		return err
	}

	c, err := net.Dial("tcp", "127.0.0.1:10001")
	if err != nil {
		return err
	}
	defer c.Close()

	e := gob.NewEncoder(c)
	d := gob.NewDecoder(c)

	err = e.Encode(data)
	if err != nil {
		return err
	}

	var ok string
	err = d.Decode(&ok)
	if err != nil {
		return err
	}
	if ok != "ok" {
		return fmt.Errorf("did not receive ok")
	}

	return nil
}

func p2pReceiveFile(ctx context.Context, node *rpc.HttpApi) ([]byte, error) {
	l, err := net.Listen("tcp", "127.0.0.1:10002")
	if err != nil {
		return nil, err
	}
	defer l.Close()
	addr := l.Addr().(*net.TCPAddr)

	targetAddr := fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", addr.Port)

	// close the old one
	_, _ = node.Request("p2p/close").
		Option("protocol", "/x/kluctl-preview-info").
		Option("target-address", targetAddr).
		Send(ctx)
	_, err = node.Request("p2p/listen", "/x/kluctl-preview-info", targetAddr).
		Send(ctx)
	if err != nil {
		return nil, err
	}

	c, err := l.Accept()
	if err != nil {
		return nil, err
	}
	defer c.Close()

	d := gob.NewDecoder(c)
	e := gob.NewEncoder(c)

	var data []byte
	err = d.Decode(&data)
	if err != nil {
		return nil, err
	}

	ok := "ok"
	err = e.Encode(&ok)
	if err != nil {
		return nil, err
	}

	return data, nil
}
