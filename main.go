package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
)

func main() {
	fmt.Println("Hello, Mac Setup!")

	// 1. Homebrew (if missing)
	if !installHomebrew() {
		log.Fatal("Homebrew setup failed; aborting")
	}

	// 2. Oh-My-Zsh (optional)
	if err := ensureOhMyZsh(); err != nil {
		fmt.Printf("oh-my-zsh install failed: %v\n", err)
	}

	// Load configuration
	var cfg Config
	if err := cfg.LoadConfig("config.yaml"); err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	fmt.Printf("Loaded %d taps, %d formula categories, %d cask categories\n",
		len(cfg.Brew.Taps), len(cfg.Brew.Formulae), len(cfg.Brew.Casks))

	// Apply configuration
	if err := ensureTaps(cfg.Brew.Taps); err != nil {
		log.Fatalf("tap step failed: %v", err)
	}
	if err := ensureFormulae(cfg.Brew.Formulae); err != nil {
		log.Fatalf("formula installation failed: %v", err)
	}
	if err := ensureCasks(cfg.Brew.Casks); err != nil {
		fmt.Printf("some cask issues occurred: %v\n", err)
	}

	// Dotfiles symlinking
	if err := ensureDotfiles(cfg.Dotfiles); err != nil {
		log.Fatalf("dotfile setup failed: %v", err)
	}

	fmt.Println("✅ Mac setup complete!")
}

// installHomebrew verifies Homebrew is present and installs it if necessary.
func installHomebrew() bool {
	if err := exec.Command("brew", "--version").Run(); err == nil {
		fmt.Println("Homebrew is already installed")
		return true
	}
	fmt.Println("Installing Homebrew…")
	cmd := exec.Command("bash", "-c", "curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh | bash")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("Homebrew installation failed: %v\n", err)
		return false
	}
	fmt.Println("Homebrew installed successfully")
	return true
}

// ensureOhMyZsh installs Oh-My-Zsh if it isn't already present.
func ensureOhMyZsh() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	omzDir := filepath.Join(homeDir, ".oh-my-zsh")
	if _, err := os.Stat(omzDir); err == nil {
		fmt.Println("Oh My Zsh already installed")
		return nil
	}
	fmt.Println("Installing Oh My Zsh…")
	cmd := exec.Command("bash", "-c", "RUNZSH=no CHSH=no KEEP_ZSHRC=yes sh -c \"$(curl -fsSL https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh)\"")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// ensureTaps adds any missing Homebrew taps.
func ensureTaps(taps []string) error {
	for _, tap := range taps {
		fmt.Printf("🔧 Tapping %s…\n", tap)
		cmd := exec.Command("brew", "tap", tap)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("tap %s: %w", tap, err)
		}
	}
	return nil
}

// ensureFormulae installs any missing Homebrew formulae.
func ensureFormulae(categories map[string][]string) error {
	var failed []string
	for cat, pkgs := range categories {
		fmt.Printf("📦 Category %s (%d)\n", cat, len(pkgs))
		for _, pkg := range pkgs {
			if err := installFormula(pkg); err != nil {
				fmt.Printf("✗ %s failed: %v\n", pkg, err)
				failed = append(failed, pkg)
			}
		}
	}
	if len(failed) > 0 {
		fmt.Printf("Some formulae failed to install: %v\n", failed)
	}
	return nil
}

// installFormula installs a single formula if not already installed.
func installFormula(pkg string) error {
	if err := exec.Command("brew", "list", "--formula", pkg).Run(); err == nil {
		fmt.Printf("• %s already installed\n", pkg)
		return nil
	}
	fmt.Printf("→ Installing formula %s\n", pkg)
	cmd := exec.Command("brew", "install", pkg)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("install %s: %w", pkg, err)
	}
	return nil
}

// ensureCasks installs any missing Homebrew casks.
func ensureCasks(categories map[string][]string) error {
	var failed []string
	for cat, pkgs := range categories {
		fmt.Printf("🍺 Cask category %s (%d)\n", cat, len(pkgs))
		for _, pkg := range pkgs {
			if err := installCask(pkg); err != nil {
				fmt.Printf("✗ %s failed: %v\n", pkg, err)
				failed = append(failed, pkg)
			}
		}
	}
	if len(failed) > 0 {
		fmt.Printf("Some casks failed to install: %v\n", failed)
	}
	return nil
}

// installCask installs a single cask if not already installed.
func installCask(pkg string) error {
	if err := exec.Command("brew", "list", "--cask", pkg).Run(); err == nil {
		fmt.Printf("• %s already installed (cask)\n", pkg)
		return nil
	}
	fmt.Printf("→ Installing cask %s\n", pkg)
	cmd := exec.Command("brew", "install", "--cask", pkg)
	output, err := cmd.CombinedOutput()
	if err != nil {
		outStr := string(output)
		if strings.Contains(outStr, "already an app at") || strings.Contains(outStr, "already installed") || strings.Contains(outStr, "It seems there is") || strings.Contains(outStr, "depends on hardware architecture") {
			fmt.Printf("• %s already present outside Homebrew, skipping\n", pkg)
			return nil
		}
		return fmt.Errorf("install cask %s failed: %v\n%s", pkg, err, outStr)
	}
	fmt.Printf("✓ installed cask %s\n", pkg)
	return nil
}

// ensureDotfiles creates symlinks for each dotfile in the repo's dotfiles directory into $HOME.
func ensureDotfiles(files []string) error {
	if len(files) == 0 {
		return nil
	}
	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}
	dotDir := filepath.Join(repoRoot, "dotfiles")
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	fmt.Println("📄 Copying dotfiles…")
	for _, name := range files {
		src := filepath.Join(dotDir, name)
		dest := filepath.Join(homeDir, name)

		data, err := os.ReadFile(src)
		if err != nil {
			fmt.Printf("• unable to read %s, skipping (%v)\n", src, err)
			continue
		}
		if err := os.WriteFile(dest, data, 0644); err != nil {
			return fmt.Errorf("write %s: %w", dest, err)
		}
		fmt.Printf("→ copied %s → %s\n", src, dest)
	}
	return nil
}

// Config structures --------------------------------------------

type Config struct {
	Brew     Brew     `yaml:"brew"`
	Dotfiles []string `yaml:"dotfiles"`
}

type Brew struct {
	Taps     []string            `yaml:"taps"`
	Formulae map[string][]string `yaml:"formulae"`
	Casks    map[string][]string `yaml:"casks"`
}

// LoadConfig reads a YAML file into the Config struct.
func (c *Config) LoadConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, c)
}
