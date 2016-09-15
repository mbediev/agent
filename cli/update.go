package lib

import (
	"crypto/tls"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/cheggaaa/pb"

	"github.com/subutai-io/base/agent/config"
	"github.com/subutai-io/base/agent/lib/container"
	"github.com/subutai-io/base/agent/lib/gpg"
	"github.com/subutai-io/base/agent/log"
)

type snap struct {
	Id        string            `json:"id"`
	Name      string            `json:"name"`
	Owner     []string          `json:"owner"`
	Version   string            `json:"version"`
	Signature map[string]string `json:"signature"`
}

func ifUpdateable(installed int) string {
	var update []snap
	var hash string

	tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	client := &http.Client{Transport: tr}
	if !config.Cdn.Allowinsecure {
		client = &http.Client{}
	}
	resp, err := client.Get("https://" + config.Cdn.Url + ":" + config.Cdn.Sslport + "/kurjun/rest/file/info?name=subutai_")
	log.Check(log.FatalLevel, "GET: https://"+config.Cdn.Url+":"+config.Cdn.Sslport+"/kurjun/rest/file/info?name=subutai_", err)
	defer resp.Body.Close()
	js, err := ioutil.ReadAll(resp.Body)
	log.Check(log.FatalLevel, "Reading response", err)
	log.Check(log.FatalLevel, "Parsing file list", json.Unmarshal(js, &update))

	for _, v := range update {
		available, err := strconv.Atoi(v.Version)
		log.Check(log.ErrorLevel, "Matching update "+v.Version, err)
		if len(config.Template.Branch) != 0 && strings.HasSuffix(v.Name, config.Template.Arch+"-"+config.Template.Branch+".snap") && installed < available && verifyAuthor(v) {
			log.Debug("Found newer snap: " + v.Name + ", " + v.Version)
			installed = available
			hash = v.Id
		} else if len(config.Template.Branch) == 0 && strings.HasSuffix(v.Name, config.Template.Arch+".snap") && installed < available && verifyAuthor(v) {
			log.Debug("Found newer snap: " + v.Name + ", " + v.Version)
			installed = available
			hash = v.Id
		}
	}

	log.Debug("Latest version: " + strconv.Itoa(installed))
	return hash
}

func verifyAuthor(p snap) bool {
	if _, ok := p.Signature["jenkins"]; !ok {
		log.Debug("Update is not owned by Subutai team, ignoring")
		return false
	}
	signedhash := gpg.VerifySignature(gpg.KurjunUserPK("jenkins"), p.Signature["jenkins"])
	if p.Id != signedhash {
		log.Debug("Signature does not match with update hash")
		return false
	}
	log.Debug("Digital signature and file integrity verified")
	return true
}

func getInstalled() string {
	f, err := ioutil.ReadFile(config.Agent.AppPrefix + "/meta/package.yaml")
	if !log.Check(log.DebugLevel, "Reading file package.yaml", err) {
		lines := strings.Split(string(f), "\n")
		for _, v := range lines {
			if strings.HasPrefix(v, "version: ") {
				if version := strings.Split(strings.TrimPrefix(v, "version: "), "-"); len(version) > 1 {
					log.Debug("Installed: " + version[1])
					return version[1]
				}
			}
		}
	}
	return "0"
}

func upgradeRh(hash string) {
	log.Info("Updating Resource host")
	file, err := os.Create("/tmp/" + hash)
	log.Check(log.FatalLevel, "Creating update file", err)
	defer file.Close()
	tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	client := &http.Client{Transport: tr}
	if !config.Cdn.Allowinsecure {
		client = &http.Client{}
	}
	resp, err := client.Get("https://" + config.Cdn.Url + ":" + config.Cdn.Sslport + "/kurjun/rest/file/get?id=" + hash)
	log.Check(log.FatalLevel, "GET: https://"+config.Cdn.Url+":"+config.Cdn.Sslport+"/kurjun/rest/file/get?id="+hash, err)
	defer resp.Body.Close()
	log.Info("Downloading snap package")
	bar := pb.New(int(resp.ContentLength)).SetUnits(pb.U_BYTES)
	bar.Start()
	rd := bar.NewProxyReader(resp.Body)

	_, err = io.Copy(file, rd)
	log.Check(log.FatalLevel, "Writing response to file", err)
	if hash != md5sum("/tmp/"+hash) {
		log.Error("Hash does not match")
	}

	log.Info("Installing update")
	log.Check(log.FatalLevel, "Installing update /tmp/"+hash,
		exec.Command("snappy", "install", "--allow-unauthenticated", "/tmp/"+hash).Run())
	log.Check(log.FatalLevel, "Removing update file /tmp/"+hash, os.Remove("/tmp/"+hash))

}

func Update(name string, check bool) {
	if !lockSubutai(name + ".update") {
		log.Error("Another update process is already running")
	}
	defer unlockSubutai()
	switch name {
	case "rh":

		installed, err := strconv.Atoi(getInstalled())
		log.Check(log.FatalLevel, "Converting installed package timestamp to int", err)

		newsnap := ifUpdateable(installed)
		if len(newsnap) == 0 {
			log.Info("No update is available")
			os.Exit(1)
		} else if check {
			log.Info("Update is available")
			os.Exit(0)
		}
		upgradeRh(newsnap)

	default:
		if !container.IsContainer(name) {
			log.Error("no such instance \"" + name + "\"")
		}
		_, err := container.AttachExec(name, []string{"apt-get", "-qq", "update", "-y", "--force-yes", "-o", "Acquire::http::Timeout=5"})
		log.Check(log.FatalLevel, "Updating apt index", err)
		output, err := container.AttachExec(name, []string{"apt-get", "-qq", "upgrade", "-y", "--force-yes", "-o", "Acquire::http::Timeout=5", "-s"})
		log.Check(log.FatalLevel, "Checking for available update", err)
		if len(output) == 0 {
			log.Info("No update is available")
			os.Exit(1)
		} else if check {
			log.Info("Update is available")
			os.Exit(0)
		}
		_, err = container.AttachExec(name, []string{"apt-get", "-qq", "upgrade", "-y", "--force-yes", "-o", "Acquire::http::Timeout=5"})
		log.Check(log.FatalLevel, "Updating container", err)
	}
}
