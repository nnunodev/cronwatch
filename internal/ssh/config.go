package ssh

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// SSHHostConfig holds the resolved settings for a Host entry in ~/.ssh/config
type SSHHostConfig struct {
	HostName     string
	User         string
	Port         int
	IdentityFile string
}

// ReadSSHConfig parses ~/.ssh/config and returns the first matching Host block
// for the given host alias. OpenSSH behavior: first match wins.
func ReadSSHConfig(host string) (*SSHHostConfig, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	p := filepath.Join(home, ".ssh", "config")
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var result *SSHHostConfig
	scanner := bufio.NewScanner(f)
	var currentHosts []string
	var current SSHHostConfig

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := strings.ToLower(fields[0])
		val := fields[1]

		if key == "host" {
			// Evaluate previous block before starting a new one
			if result == nil {
				for _, h := range currentHosts {
					if matchSSHHost(host, h) {
						result = &current
						break
					}
				}
			}
			currentHosts = fields[1:]
			current = SSHHostConfig{}
			continue
		}

		// Only accumulate settings inside a Host block
		if len(currentHosts) == 0 {
			continue
		}
		// First-match-wins: stop parsing once we have a match
		if result != nil {
			continue
		}

		switch key {
		case "hostname":
			current.HostName = val
		case "user":
			current.User = val
		case "port":
			if port, err := strconv.Atoi(val); err == nil {
				current.Port = port
			}
		case "identityfile":
			current.IdentityFile = expandTilde(val)
		}
	}

	// Evaluate final block
	if result == nil {
		for _, h := range currentHosts {
			if matchSSHHost(host, h) {
				result = &current
				break
			}
		}
	}

	if result == nil {
		return nil, fmt.Errorf("no Host entry for %s", host)
	}
	return result, nil
}

// matchSSHHost checks if target matches a single OpenSSH Host pattern.
// Supports literal match and simple prefix wildcard (e.g. "hyperion*").
// Ignores the global "*" wildcard so we don't return a catch-all block.
func matchSSHHost(target, pattern string) bool {
	// Skip the global catch-all
	if pattern == "*" {
		return false
	}
	if strings.Contains(pattern, "*") {
		parts := strings.Split(pattern, "*")
		if !strings.HasPrefix(target, parts[0]) {
			return false
		}
		rest := target[len(parts[0]):]
		for i := 1; i < len(parts); i++ {
			idx := strings.Index(rest, parts[i])
			if idx == -1 {
				return false
			}
			rest = rest[idx+len(parts[i]):]
		}
		return true
	}
	return pattern == target
}

func expandTilde(p string) string {
	if !strings.HasPrefix(p, "~/") {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	return filepath.Join(home, p[2:])
}
