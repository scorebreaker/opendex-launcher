package main

import (
	"fmt"
	"github.com/opendexnetwork/opendex-launcher/core"
	"github.com/mitchellh/go-homedir"
	"os"
	"path/filepath"
	"runtime"
)

func main() {
	homeDir, err := GetHomeDir()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	r, err := core.NewLauncher(homeDir)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	network := GetNetwork()
	networkDir := filepath.Join(homeDir, network)
	branch := GetBranch()

	err = r.Start(branch, network, networkDir, os.Args...)
	if err != nil {
		fmt.Printf("ERROR: %s\n", err)
		os.Exit(1)
	}
}

func GetHomeDir() (string, error) {
	homeDir, err := homedir.Dir()
	if err != nil {
		panic(err)
	}
	switch runtime.GOOS {
	case "linux":
		return filepath.Join(homeDir, ".opendex-docker"), nil
	case "darwin":
		return filepath.Join(homeDir, "Library", "Application Support", "OpendexDocker"), nil
	case "windows":
		return filepath.Join(homeDir, "AppData", "Local", "OpendexDocker"), nil
	default:
		return "", fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

func GetNetwork() string {
	if value, ok := os.LookupEnv("NETWORK"); ok {
		return value
	}
	return "mainnet"
}

func GetBranch() string {
	if value, ok := os.LookupEnv("BRANCH"); ok {
		return value
	}
	return "master"
}
