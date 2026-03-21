package docker

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
)

func buildTag(baseImage string, features map[string]interface{}, containerName string) string {
	h := sha256.New()
	h.Write([]byte(baseImage))

	keys := make([]string, 0, len(features))
	for k := range features {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte(fmt.Sprintf("%v", features[k])))
	}

	short := fmt.Sprintf("%x", h.Sum(nil)[:6])
	return fmt.Sprintf("devc/%s:%s", containerName, short)
}

func generateDockerfile(baseImage string, features map[string]interface{}) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("FROM %s\n\n", baseImage))
	b.WriteString("USER root\n\n")

	keys := make([]string, 0, len(features))
	for k := range features {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, ref := range keys {
		opts := featureOpts(features[ref])
		installCmd := featureInstallCommand(ref, opts)
		if installCmd != "" {
			b.WriteString(fmt.Sprintf("# Feature: %s\n", ref))
			b.WriteString(fmt.Sprintf("RUN %s\n\n", installCmd))
		}
	}

	b.WriteString("# Restore non-root user if available\n")
	b.WriteString("ARG USERNAME=vscode\n")
	b.WriteString("RUN id -u ${USERNAME} 2>/dev/null && chown -R ${USERNAME} /home/${USERNAME} || true\n")

	return b.String()
}

func featureOpts(v interface{}) map[string]string {
	opts := make(map[string]string)
	switch val := v.(type) {
	case map[string]interface{}:
		for k, v := range val {
			opts[k] = fmt.Sprintf("%v", v)
		}
	case map[string]string:
		for k, v := range val {
			opts[k] = v
		}
	}
	return opts
}

func featureInstallCommand(ref string, opts map[string]string) string {
	name := extractFeatureName(ref)
	version := opts["version"]

	switch name {
	case "node":
		nodeVersion := "lts"
		if version != "" {
			nodeVersion = version
		}
		return fmt.Sprintf(
			"apt-get update && apt-get install -y curl && "+
				"curl -fsSL https://deb.nodesource.com/setup_%s.x | bash - && "+
				"apt-get install -y nodejs && "+
				"npm install -g npm@latest && "+
				"apt-get clean && rm -rf /var/lib/apt/lists/*",
			nodeVersion,
		)

	case "python":
		pythonVersion := "3"
		if version != "" {
			pythonVersion = version
		}
		return fmt.Sprintf(
			"apt-get update && "+
				"apt-get install -y python%s python3-pip python3-venv && "+
				"apt-get clean && rm -rf /var/lib/apt/lists/*",
			pythonVersion,
		)

	case "go", "golang":
		goVersion := "latest"
		if version != "" {
			goVersion = version
		}
		if goVersion == "latest" {
			return "apt-get update && apt-get install -y curl && " +
				"curl -fsSL https://go.dev/dl/$(curl -fsSL 'https://go.dev/VERSION?m=text' | head -1).linux-$(dpkg --print-architecture).tar.gz | " +
				"tar -C /usr/local -xzf - && " +
				"ln -s /usr/local/go/bin/go /usr/local/bin/go && " +
				"apt-get clean && rm -rf /var/lib/apt/lists/*"
		}
		return fmt.Sprintf(
			"apt-get update && apt-get install -y curl && "+
				"curl -fsSL https://go.dev/dl/go%s.linux-$(dpkg --print-architecture).tar.gz | "+
				"tar -C /usr/local -xzf - && "+
				"ln -s /usr/local/go/bin/go /usr/local/bin/go && "+
				"apt-get clean && rm -rf /var/lib/apt/lists/*",
			goVersion,
		)

	case "rust":
		return "apt-get update && apt-get install -y curl build-essential && " +
			"curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y && " +
			"apt-get clean && rm -rf /var/lib/apt/lists/*"

	case "docker-in-docker":
		return "apt-get update && apt-get install -y curl && " +
			"curl -fsSL https://get.docker.com | sh && " +
			"apt-get clean && rm -rf /var/lib/apt/lists/*"

	case "git":
		return "apt-get update && apt-get install -y git && " +
			"apt-get clean && rm -rf /var/lib/apt/lists/*"

	case "github-cli":
		return "apt-get update && apt-get install -y curl && " +
			"curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg && " +
			"echo 'deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main' > /etc/apt/sources.list.d/github-cli.list && " +
			"apt-get update && apt-get install -y gh && " +
			"apt-get clean && rm -rf /var/lib/apt/lists/*"

	case "common-utils":
		return "apt-get update && " +
			"apt-get install -y sudo curl wget ca-certificates gnupg2 jq less vim nano htop procps net-tools && " +
			"apt-get clean && rm -rf /var/lib/apt/lists/*"

	case "java":
		javaVersion := "17"
		if version != "" {
			javaVersion = version
		}
		return fmt.Sprintf(
			"apt-get update && apt-get install -y openjdk-%s-jdk && "+
				"apt-get clean && rm -rf /var/lib/apt/lists/*",
			javaVersion,
		)

	default:
		return fmt.Sprintf(
			"echo 'Feature %s: manual installation may be required' && "+
				"apt-get update && apt-get install -y %s 2>/dev/null || "+
				"echo 'Could not auto-install feature %s'",
			ref, name, ref,
		)
	}
}

func extractFeatureName(ref string) string {
	if idx := strings.LastIndex(ref, ":"); idx != -1 {
		if strings.Contains(ref[:idx], "/") || !strings.Contains(ref, "/") {
			ref = ref[:idx]
		}
	}
	if idx := strings.LastIndex(ref, "/"); idx != -1 {
		return ref[idx+1:]
	}
	return ref
}
