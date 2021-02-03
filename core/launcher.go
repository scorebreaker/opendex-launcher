package core

import (
	"fmt"
	"github.com/opendexnetwork/opendex-launcher/config"
	"github.com/opendexnetwork/opendex-launcher/logging"
	"github.com/sirupsen/logrus"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type Launcher struct {
	logger *logrus.Entry

	homeDir     string
	launcherDir string
	versionsDir string

	configFile string

	GitHub *GitHub
}

func NewLauncher(homeDir string) (*Launcher, error) {
	configFile := filepath.Join(homeDir, "opendexd-docker.conf")
	cfg, err := initConfig(configFile)
	if err != nil {
		return nil, err
	}

	launcherDir := filepath.Join(homeDir, "launcher")
	versionsDir := filepath.Join(launcherDir, "versions")

	logrus.StandardLogger().SetLevel(logrus.DebugLevel)
	logrus.StandardLogger().SetFormatter(&logging.Formatter{})

	r := Launcher{
		homeDir:     homeDir,
		launcherDir: launcherDir,
		versionsDir: versionsDir,
		logger:      logrus.NewEntry(logrus.StandardLogger()).WithField("name", "core"),
		configFile:  configFile,
		GitHub:      NewGitHub(cfg.GitHub.AccessToken),
	}

	if err := r.init(); err != nil {
		return nil, err
	}

	return &r, nil
}

func initConfig(configFile string) (*config.Config, error) {
	var cfg *config.Config

	f, err := os.Open(configFile)
	if err != nil {
		cfg = &config.Config{}
	} else {
		defer f.Close()
		cfg, err = config.ParseConfig(f)
		if err != nil {
			return nil, fmt.Errorf("parse config: %w", err)
		}
	}

	return cfg, nil
}

func (t *Launcher) init() error {
	if _, err := os.Stat(t.launcherDir); os.IsNotExist(err) {
		if err := os.MkdirAll(t.launcherDir, 0755); err != nil {
			return fmt.Errorf("mkdir: %w", err)
		}
	}
	err := os.Chdir(t.launcherDir)
	if err != nil {
		return fmt.Errorf("chdir: %w", err)
	}

	return nil
}

func (t *Launcher) Run(name string, args ...string) error {
	t.logger.Debugf("[run] %s %s", name, strings.Join(args, " "))
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (t *Launcher) Start(branch string, network string, networkDir string, args ...string) error {
	commit, err := t.GitHub.GetHeadCommit(branch)
	if err != nil {
		return fmt.Errorf("git head commit: %w", err)
	}

	t.logger.Debugf("Start launcher with branch=%s(%s), network=%s, networkDir=%s", branch, commit, network, networkDir)

	if _, err := os.Stat(t.versionsDir); err != nil {
		if err := os.Mkdir(t.versionsDir, 0755); err != nil {
			return fmt.Errorf("mkdir: %w", err)
		}
	}
	if err := os.Chdir(t.versionsDir); err != nil {
		return fmt.Errorf("chdir: %w", err)
	}

	var commitLauncher string
	if runtime.GOOS == "windows" {
		commitLauncher = filepath.Join(commit, "launcher.exe")
	} else {
		commitLauncher = filepath.Join(commit, "launcher")
	}

	if _, err := os.Stat(commitLauncher); os.IsNotExist(err) {
		if err := t.GitHub.DownloadLatestBinary(branch, commit); err != nil {
			return fmt.Errorf("download latest binary: %w", err)
		}
	} else {
		if err := os.Chdir(commit); err != nil {
			return fmt.Errorf("chdir: %w", err)
		}
	}

	if err := os.Setenv("NETWORK", network); err != nil {
		return fmt.Errorf("setenv: %w", err)
	}

	if err := os.Setenv("NETWORK_DIR", networkDir); err != nil {
		return fmt.Errorf("setenv: %w", err)
	}

	var launcher string
	if runtime.GOOS == "windows" {
		launcher = ".\\launcher.exe"

	} else {
		launcher = "./launcher"
	}

	if runtime.GOOS != "windows" {
		// check if binary launcher is executable
		info, _ := os.Stat(launcher)
		mode := info.Mode()
		if mode&0100 == 0 {
			err := os.Chmod(launcher, 0755)
			if err != nil {
				return fmt.Errorf("chmod: %w", err)
			}
		}
	}

	if err := t.Run(launcher, args[1:]...); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return fmt.Errorf("run: %w", err)
	}

	return nil
}
