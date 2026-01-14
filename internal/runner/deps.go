package runner

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
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


// EnsurePythonPackages checks that a Python executable is available and that the
// requested pip packages are installed. If a package is missing it prompts the
// user and can install it using `python -m pip install`.
func EnsurePythonPackages(packages []string, pythonExec string) error {
	if pythonExec == "" {
		pythonExec = os.Getenv("CT_PYTHON")
	}
	if pythonExec == "" {
		pythonExec = "python3"
	}

	if _, err := exec.LookPath(pythonExec); err != nil {
		fmt.Fprintf(os.Stderr, "python not found: %s\n", pythonExec)
		consent, err := promptConsent("Install Python now via system package manager?")
		if err != nil {
			return err
		}
		if !consent {
			return fmt.Errorf("missing python executable: %s", pythonExec)
		}
		return fmt.Errorf("please install Python and re-run")
	}

	for _, pkg := range packages {
		cmd := exec.Command(pythonExec, "-m", "pip", "show", pkg)
		if err := cmd.Run(); err == nil {
			continue
		}

		fmt.Fprintf(os.Stderr, "python package %s is not installed\n", pkg)
		consent, err := promptConsent(fmt.Sprintf("Install %s with %s -m pip install %s?", pkg, pythonExec, pkg))
		if err != nil {
			return err
		}
		if !consent {
			return fmt.Errorf("missing python package: %s", pkg)
		}

		install := exec.Command(pythonExec, "-m", "pip", "install", pkg)
		install.Stdout = os.Stdout
		install.Stderr = os.Stderr
		install.Stdin = os.Stdin
		if err := install.Run(); err != nil {
			return fmt.Errorf("failed to install python package %s", pkg)
		}
	}
	return nil
}


// Signed-off-by: ronikoz
