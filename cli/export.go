package lib

import (
	"github.com/subutai-io/base/agent/cli/lib"
	"github.com/subutai-io/base/agent/config"
	"github.com/subutai-io/base/agent/lib/container"
	"github.com/subutai-io/base/agent/lib/fs"
	"github.com/subutai-io/base/agent/lib/template"
	"github.com/subutai-io/base/agent/log"
	"os"
	"runtime"
)

// cfg declared in promote.go
// LxcExport exports the given name if it suits the needs.
func LxcExport(name, version string) {
	srcver := container.GetConfigItem(config.Agent.LxcPrefix+name+"/config", "subutai.template.version")
	if len(version) == 0 {
		version = srcver
	}
	dst := config.Agent.LxcPrefix + "tmpdir/" + name +
		"-subutai-template_" + version + "_" + runtime.GOARCH

	if !container.IsTemplate(name) {
		LxcPromote(name)
	}
	// check: parent is template
	parent := container.GetParent(name)
	if !container.IsTemplate(parent) {
		log.Error("Parent " + parent + " is not a template")
	}

	os.MkdirAll(dst, 0755)
	os.MkdirAll(dst+"/deltas", 0755)
	os.MkdirAll(dst+"/diff", 0755)

	fs.Send(config.Agent.LxcPrefix+parent+"/rootfs", config.Agent.LxcPrefix+name+"/rootfs", dst+"/deltas/rootfs.delta")
	fs.Send(config.Agent.LxcPrefix+parent+"/home", config.Agent.LxcPrefix+name+"/home", dst+"/deltas/home.delta")
	fs.Send(config.Agent.LxcPrefix+parent+"/opt", config.Agent.LxcPrefix+name+"/opt", dst+"/deltas/opt.delta")
	fs.Send(config.Agent.LxcPrefix+parent+"/var", config.Agent.LxcPrefix+name+"/var", dst+"/deltas/var.delta")

	// changeConfigFile(name, packageVersion, dst)
	container.SetContainerConf(name, [][]string{
		{"subutai.template.package", dst + ".tar.gz"},
		{"subutai.template.version", version},
	})

	src := config.Agent.LxcPrefix + name
	lib.CopyFile(src+"/fstab", dst+"/fstab")
	lib.CopyFile(src+"/config", dst+"/config")
	lib.CopyFile(src+"/packages", dst+"/packages")
	if parent != name {
		lib.CopyFile(src+"/diff/var.diff", dst+"/diff/var.diff")
		lib.CopyFile(src+"/diff/opt.diff", dst+"/diff/opt.diff")
		lib.CopyFile(src+"/diff/home.diff", dst+"/diff/home.diff")
		lib.CopyFile(src+"/diff/rootfs.diff", dst+"/diff/rootfs.diff")
	}

	container.SetContainerConf(name, [][]string{
		{"subutai.template.package", config.Agent.LxcPrefix + "tmpdir/" + name +
			"-subutai-template_" + srcver + "_" + runtime.GOARCH + ".tar.gz"},
		{"subutai.template.version", srcver},
	})

	template.Tar(dst, dst+".tar.gz")
	log.Check(log.FatalLevel, "Remove tmpdir", os.RemoveAll(dst))
	log.Info(name + " exported to " + dst + ".tar.gz")
}
