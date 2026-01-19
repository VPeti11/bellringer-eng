package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

const (
	ProgramName = "bellringer"
	AuthorName  = "VPeti11"
	GitRepoURL  = "https://github.com/VPeti11/bellringer.git"
)

func main() {
	CheckLinux()

	ShowWelcome()

	pm := DetectPackageManager()
	if pm == "" {
		log.Fatal("No supported package manager found (apt/dnf/pacman)")
	}

	fmt.Println("Installing dependencies...")
	if err := InstallDependencies(pm); err != nil {
		log.Fatal(err)
	}

	repoDir, err := BellringerDir()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Cloning repository...")
	if err := CloneRepo(repoDir); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Removing precompiled binary (if any)...")
	if err := RemoveExistingBinary(repoDir); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Building bellringer...")
	if err := BuildBinary(repoDir); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Linking to /usr/bin...")
	if err := LinkBinary(repoDir); err != nil {
		log.Fatal(err)
	}

	fmt.Println("\nbellringer installed successfully!")
	fmt.Println("Run it with: bellringer")
}

func CheckLinux() {
	if runtime.GOOS != "linux" {
		log.Fatal("This installer only supports Linux")
	}
}

func BellringerDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".bellringer"), nil
}

func CloneRepo(dir string) error {
	_ = os.RemoveAll(dir)

	cmd := exec.Command("git", "clone", GitRepoURL, dir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RemoveExistingBinary(repoDir string) error {
	bin := filepath.Join(repoDir, ProgramName)
	if _, err := os.Stat(bin); err == nil {
		return os.Remove(bin)
	}
	return nil
}

func BuildBinary(repoDir string) error {
	cmd := exec.Command("go", "build", "-o", ProgramName)
	cmd.Dir = repoDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func LinkBinary(repoDir string) error {
	src := filepath.Join(repoDir, ProgramName)
	dst := filepath.Join("/usr/bin", ProgramName)

	_ = exec.Command("sudo", "rm", "-f", dst).Run()

	cmd := exec.Command("sudo", "ln", "-s", src, dst)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func DetectPackageManager() string {
	switch {
	case CommandExists("apt"):
		return "apt"
	case CommandExists("dnf"):
		return "dnf"
	case CommandExists("pacman"):
		return "pacman"
	default:
		return ""
	}
}

func CommandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

func InstallDependencies(pm string) error {
	switch pm {
	case "apt":
		exec.Command("sudo", "apt", "update").Run()
		return exec.Command("sudo", "apt", "install", "-y", "git", "go").Run()
	case "dnf":
		return exec.Command("sudo", "dnf", "install", "-y", "git", "go").Run()
	case "pacman":
		return exec.Command("sudo", "pacman", "-Syu", "--noconfirm", "git", "go").Run()
	}
	return nil
}

func ShowWelcome() {
	clear()
	fmt.Printf("Welcome to %s installer\n", ProgramName)
	fmt.Printf("Made by %s\n\n", AuthorName)
	fmt.Print("Press Enter to continue...")
	bufio.NewReader(os.Stdin).ReadBytes('\n')
	clear()
}

func clear() {
	cmd := exec.Command("clear")
	cmd.Stdout = os.Stdout
	cmd.Run()
}
