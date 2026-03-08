package runner

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

type Installer struct {
	Name    string
	Command []string
}

type Dependency struct {
	Name        string
	CheckCmd    string
	Installers  map[string][]Installer
	Description string
}

func EnsureDependencies(deps []Dependency) error {
	for _, dep := range deps {
		if _, err := exec.LookPath(dep.CheckCmd); err == nil {
			continue
		}

		fmt.Fprintf(os.Stderr, "%s is not installed (%s).\n", dep.Name, dep.Description)
		consent, err := promptConsent("Install now?")
		if err != nil {
			return err
		}
		if !consent {
			return fmt.Errorf("missing dependency: %s", dep.Name)
		}

		if err := installDependency(dep); err != nil {
			return err
		}
	}
	return nil
}

func installDependency(dep Dependency) error {
	installers := dep.Installers[runtime.GOOS]
	if len(installers) == 0 {
		return fmt.Errorf("no installer defined for %s on %s", dep.Name, runtime.GOOS)
	}

	for _, installer := range installers {
		if _, err := exec.LookPath(installer.Command[0]); err != nil {
			continue
		}
		fmt.Fprintf(os.Stderr, "Running %s: %s\n", installer.Name, strings.Join(installer.Command, " "))
		cmd := exec.Command(installer.Command[0], installer.Command[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("installer failed: %s", installer.Name)
		}
		return nil
	}

	return errors.New("no supported package manager found on PATH")
}

func promptConsent(message string) (bool, error) {
	fmt.Fprintf(os.Stderr, "%s [y/N]: ", message)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	line = strings.TrimSpace(strings.ToLower(line))
	return line == "y" || line == "yes", nil
}

func BaseInstallers(pkg string, wingetID string, chocoPkg string) map[string][]Installer {
	return map[string][]Installer{
		"darwin": {
			{Name: "brew", Command: []string{"brew", "install", pkg}},
		},
		"linux": {
			{Name: "apt", Command: []string{"sudo", "apt-get", "install", "-y", pkg}},
			{Name: "dnf", Command: []string{"sudo", "dnf", "install", "-y", pkg}},
			{Name: "pacman", Command: []string{"sudo", "pacman", "-S", "--noconfirm", pkg}},
		},
		"windows": {
			{Name: "winget", Command: []string{"winget", "install", "--id", wingetID, "-e"}},
			{Name: "choco", Command: []string{"choco", "install", "-y", chocoPkg}},
		},
	}
}

func pythonVenvPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".cache", "ct_plugins", "env")
}

func stampPath() string {
	return filepath.Join(pythonVenvPath(), ".installed_pkgs")
}

func readStamp() map[string]bool {
	data, err := os.ReadFile(stampPath())
	if err != nil {
		return map[string]bool{}
	}
	pkgs := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			pkgs[line] = true
		}
	}
	return pkgs
}

func writeStamp(pkgs []string) {
	sorted := make([]string, len(pkgs))
	copy(sorted, pkgs)
	sort.Strings(sorted)
	_ = os.WriteFile(stampPath(), []byte(strings.Join(sorted, "\n")+"\n"), 0600)
}

// GetVenvPython returns the path to the python executable inside the virtual environment
func GetVenvPython() string {
	venv := pythonVenvPath()
	if venv == "" {
		return ""
	}
	if runtime.GOOS == "windows" {
		return filepath.Join(venv, "Scripts", "python.exe")
	}
	return filepath.Join(venv, "bin", "python3")
}

// EnsurePythonPackages checks that a Python executable is available, creates a virtual
// environment if needed, and ensures the requested pip packages are installed inside it.
func EnsurePythonPackages(packages []string, systemPython string) error {
	if systemPython == "" {
		systemPython = os.Getenv("CT_PYTHON")
	}
	if systemPython == "" {
		systemPython = "python3"
	}

	if _, err := exec.LookPath(systemPython); err != nil {
		fmt.Fprintf(os.Stderr, "system python not found: %s\n", systemPython)
		consent, err := promptConsent("Install Python now via system package manager?")
		if err != nil {
			return err
		}
		if !consent {
			return fmt.Errorf("missing python executable: %s", systemPython)
		}
		return fmt.Errorf("please install Python and re-run")
	}

	venvPython := GetVenvPython()
	if venvPython == "" {
		return fmt.Errorf("could not determine venv path")
	}

	// Create venv if the python executable doesn't exist
	if _, err := os.Stat(venvPython); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "creating python virtual environment...\n")
		cmd := exec.Command(systemPython, "-m", "venv", pythonVenvPath())
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to create virtual environment: %v", err)
		}

		// Upgrade pip inside venv quietly
		exec.Command(venvPython, "-m", "pip", "install", "--upgrade", "pip").Run()

		// Remove stamp so it gets rebuilt for the fresh venv
		_ = os.Remove(stampPath())
	}

	stamp := readStamp()
	var newPkgs []string

	for _, pkg := range packages {
		if stamp[pkg] {
			continue // already installed per stamp, skip pip show
		}
		// Check if installed but not yet in stamp
		checkCmd := exec.Command(venvPython, "-m", "pip", "show", pkg)
		if err := checkCmd.Run(); err == nil {
			newPkgs = append(newPkgs, pkg) // already installed, add to stamp
			continue
		}
		// Install it
		fmt.Fprintf(os.Stderr, "installing python package: %s\n", pkg)
		install := exec.Command(venvPython, "-m", "pip", "install", pkg)
		install.Stdout = os.Stdout
		install.Stderr = os.Stderr
		install.Stdin = os.Stdin
		if err := install.Run(); err != nil {
			return fmt.Errorf("failed to install python package %s", pkg)
		}
		newPkgs = append(newPkgs, pkg)
	}

	// Rebuild stamp with all known packages
	if len(newPkgs) > 0 {
		allPkgs := make([]string, 0, len(stamp)+len(newPkgs))
		for p := range stamp {
			allPkgs = append(allPkgs, p)
		}
		allPkgs = append(allPkgs, newPkgs...)
		writeStamp(allPkgs)
	}

	return nil
}

// Signed-off-by: ronikoz
