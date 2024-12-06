package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"filippo.io/age"
	"flag"
	"fmt"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	dutil "github.com/libp2p/go-libp2p/p2p/discovery/util"
)

var modeFlag string
var topicFlag string
var ipfsId string
var prNumber int
var ageKeyFile string
var agePubKey string
var repoName string
var outFile string

func ParseFlags() error {
	flag.StringVar(&modeFlag, "mode", "", "Mode")
	flag.StringVar(&topicFlag, "topic", "", "pubsub topic")
	flag.StringVar(&ipfsId, "ipfs-id", "", "IPFS ID")
	flag.IntVar(&prNumber, "pr-number", 0, "PR number")
	flag.StringVar(&ageKeyFile, "age-key-file", "", "AGE key file")
	flag.StringVar(&agePubKey, "age-pub-key", "", "AGE pubkey")
	flag.StringVar(&repoName, "repo-name", "", "Repo name")
	flag.StringVar(&outFile, "out-file", "", "Output file")
	flag.Parse()

	return nil
}

func main() {
	//logging.SetLogLevel("*", "INFO")

	err := ParseFlags()
	if err != nil {
		panic(err)
	}

	if topicFlag == "test" {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Enter num: ")
		text, _ := reader.ReadString('\n')
		topicFlag = "my-test-topic-" + strings.TrimSpace(text)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	var h host.Host
	h, err = libp2p.New(
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"),
		libp2p.EnableRelay(),
		libp2p.EnableNATService(),
		libp2p.NATPortMap(),
		libp2p.ForceReachabilityPrivate(),
		libp2p.EnableAutoRelayWithPeerSource(findRelayPeers(func() host.Host {
			return h
		})))
	if err != nil {
		panic(err)
	}

	log.Infof("own ID: %s", h.ID().String())

	kademliaDHT, err := initDHT(ctx, h)
	if err != nil {
		log.Error(err)
		log.Exit(1)
	}
	discovery := drouting.NewRoutingDiscovery(kademliaDHT)

	switch modeFlag {
	case "publish":
		err = doPublish(ctx, h, discovery)
	case "subscribe":
		err = doSubscribe(ctx, h, discovery)
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

func initDHT(ctx context.Context, h host.Host) (*dht.IpfsDHT, error) {
	// Start a DHT, for use in peer discovery. We can't just make a new DHT
	// client because we want each peer to maintain its own local copy of the
	// DHT, so that the bootstrapping node of the DHT can go down without
	// inhibiting future peer discovery.
	kademliaDHT, err := dht.New(ctx, h)
	if err != nil {
		return nil, err
	}
	if err = kademliaDHT.Bootstrap(ctx); err != nil {
		return nil, err
	}
	var wg sync.WaitGroup
	for _, peerAddr := range dht.DefaultBootstrapPeers {
		peerinfo, _ := peer.AddrInfoFromP2pAddr(peerAddr)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := h.Connect(ctx, *peerinfo); err != nil {
				log.Info("Bootstrap warning:", err)
			} else {
				log.Infof("Connected to bootstrap peer: %s", peerinfo.String())
			}
		}()
	}
	wg.Wait()

	return kademliaDHT, nil
}

func findRelayPeers(h func() host.Host) func(ctx context.Context, num int) <-chan peer.AddrInfo {
	return func(ctx context.Context, num int) <-chan peer.AddrInfo {
		ch := make(chan peer.AddrInfo)
		go func() {
			sent := 0
		outer:
			for {
				for _, id := range h().Peerstore().PeersWithAddrs() {
					if sent >= num {
						break
					}
					protos, err := h().Peerstore().GetProtocols(id)
					if err != nil {
						continue
					}
					for _, proto := range protos {
						if strings.HasPrefix(string(proto), "/libp2p/circuit/relay/") {
							ch <- peer.AddrInfo{
								ID:    id,
								Addrs: h().Peerstore().Addrs(id),
							}
							sent++
							break
						}
					}
				}
				select {
				case <-time.After(time.Second):
					continue
				case <-ctx.Done():
					break outer
				}
			}
			close(ch)
		}()
		return ch
	}
}

type workflowInfo struct {
	PrNumber    int    `json:"prNumber"`
	IpfsId      string `json:"ipfsId"`
	GithubToken string `json:"githubToken"`
}

func doPublish(ctx context.Context, h host.Host, discovery *drouting.RoutingDiscovery) error {
	info := workflowInfo{
		PrNumber:    prNumber,
		GithubToken: os.Getenv("GITHUB_TOKEN"),
		IpfsId:      ipfsId,
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

	for {
		peersCh, err := discovery.FindPeers(ctx, topicFlag)
		if err != nil {
			return err
		}
		didSend := false
		for peer := range peersCh {
			if peer.ID == h.ID() {
				continue // No self connection
			}

			log.Infof("Trying %s with addrs %v", peer.ID, peer.Addrs)

			err = h.Connect(ctx, peer)
			if err != nil {
				log.Info(err)
				continue
			}

			err = sendFile(ctx, h, peer.ID, b)
			if err != nil {
				log.Warn(err)
				continue
			}
			didSend = true
		}
		if didSend {
			break
		}
	}

	log.Info("Done sending info.")

	return nil
}

func doSubscribe(ctx context.Context, h host.Host, discovery *drouting.RoutingDiscovery) error {
	doneCh := make(chan bool)
	h.SetStreamHandler("/x/kluctl-preview-info", func(s network.Stream) {
		defer s.Close()

		enc := gob.NewEncoder(s)
		dec := gob.NewDecoder(s)

		var b []byte
		err := dec.Decode(&b)
		if err != nil {
			log.Infof("Receive failed: %v", err)
			return
		}

		err = handleInfo(ctx, b)
		isDone := err == nil
		if err != nil {
			log.Infof("handle failed: %v", err)
			_ = enc.Encode("not ok")
		} else {
			err = enc.Encode("ok")
			if err != nil {
				log.Infof("Sending ok failed: %v", err)
				return
			}
		}

		var closeMsg string
		err = dec.Decode(&closeMsg)
		if err != nil {
			log.Infof("Receiving close msg failed: %v", err)
		}

		if isDone {
			doneCh <- true
		}
	})

	dutil.Advertise(ctx, discovery, topicFlag)
	<-doneCh
	h.RemoveStreamHandler("/x/kluctl-preview-info")

	return nil
}

func handleInfo(ctx context.Context, data []byte) error {
	idsBytes, err := os.ReadFile(ageKeyFile)
	if err != nil {
		return err
	}
	ageIds, err := age.ParseIdentities(bytes.NewReader(idsBytes))
	if err != nil {
		return err
	}
	d, err := age.Decrypt(bytes.NewReader(data), ageIds...)
	if err != nil {
		return err
	}

	w := bytes.NewBuffer(nil)
	_, err = io.Copy(w, d)
	if err != nil {
		return err
	}
	data = w.Bytes()

	var info workflowInfo
	err = json.Unmarshal(data, &info)
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

	data, err = json.Marshal(&info)
	if err != nil {
		return err
	}
	err = os.WriteFile(outFile, data, 0o600)
	if err != nil {
		return err
	}
	return nil
}

func doGithubRequest(ctx context.Context, method string, url string, body string, token string) ([]byte, error) {
	log.Infof("request: %s %s", method, url)

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

func sendFile(ctx context.Context, h host.Host, ipfsId peer.ID, data []byte) error {
	s, err := h.NewStream(ctx, ipfsId, "/x/kluctl-preview-info")
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", ipfsId.String(), err)
	}
	defer s.Close()

	enc := gob.NewEncoder(s)
	dec := gob.NewDecoder(s)

	err = enc.Encode(data)
	if err != nil {
		return fmt.Errorf("failed to send msg: %w", err)
	}

	var ok string
	err = dec.Decode(&ok)
	if err != nil {
		return fmt.Errorf("failed to read ok: %w", err)
	}
	_ = enc.Encode("close")
	if ok != "ok" {
		return fmt.Errorf("received '%s' instead of ok", ok)
	}

	return nil
}
