package docker

import (
	"crypto/sha256"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

// Compiled regexps for OCI reference component validation.
// These reject shell metacharacters that could allow command injection
// when components are interpolated into Dockerfile RUN commands.
var (
	ociRegistryRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)
	ociRepoRe     = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*(/[a-z0-9][a-z0-9._-]*)*$`)
	ociTagRe      = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)
	featureNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._+-]*$`)
	envKeyRe      = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)
)

// validateOCIRef checks that all components of an OCI reference are safe
// for interpolation into shell commands. Returns an error if any component
// contains characters outside the expected safe set.
func validateOCIRef(registry, repo, tag string) error {
	if !ociRegistryRe.MatchString(registry) {
		return fmt.Errorf("invalid OCI registry %q: only alphanumeric characters, dots, and hyphens are allowed", registry)
	}
	if !ociRepoRe.MatchString(repo) {
		return fmt.Errorf("invalid OCI repository %q: only lowercase alphanumeric characters, dots, hyphens, and path separators are allowed", repo)
	}
	if !ociTagRe.MatchString(tag) {
		return fmt.Errorf("invalid OCI tag %q: only alphanumeric characters, dots, and hyphens are allowed", tag)
	}
	return nil
}

func buildTag(baseImage string, features map[string]interface{}, containerName string) string {
	// Hash the generated Dockerfile content so that code changes to install
	// commands (e.g. switching from apt-get to OCI fetch) bust the cache.
	dockerfile := generateDockerfile(baseImage, features)

	h := sha256.New()
	h.Write([]byte(dockerfile))

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
		if isOCIFeature(ref) {
			return ociFeatureInstallCommand(ref, opts)
		}
		// Validate the extracted package name before using it in the apt-get command.
		// Feature names from untrusted devcontainer.json could contain shell
		// metacharacters that would allow command injection.
		if !featureNameRe.MatchString(name) {
			_, _ = fmt.Fprintf(os.Stderr, "warning: skipping feature with unsafe name %q\n", ref)
			return ""
		}
		return fmt.Sprintf(
			"echo 'Feature %s: manual installation may be required' && "+
				"apt-get update && apt-get install -y %s 2>/dev/null || "+
				"echo 'Could not auto-install feature %s'",
			name, name, name,
		)
	}
}

// isOCIFeature returns true if the feature reference looks like an OCI artifact (ghcr.io/...).
func isOCIFeature(ref string) bool {
	return strings.Contains(ref, "ghcr.io/") || strings.Contains(ref, ".azurecr.io/") || strings.Contains(ref, ".pkg.dev/")
}

// ociFeatureInstallCommand generates a shell command that pulls an OCI devcontainer
// feature and runs its install.sh. GHCR (and other registries) require a bearer
// token even for public packages, so the command first obtains an anonymous token
// from the registry's token endpoint.
func ociFeatureInstallCommand(ref string, opts map[string]string) string {
	registry, repo, tag := parseOCIRef(ref)

	if err := validateOCIRef(registry, repo, tag); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "warning: skipping OCI feature %q: %v\n", ref, err)
		return ""
	}

	// Build environment variables from feature options.
	// Option keys are validated against envKeyRe before being exported to prevent
	// environment variable name injection into the install.sh shell environment.
	var envExports string
	for k, v := range opts {
		key := strings.ToUpper(k)
		if !envKeyRe.MatchString(key) {
			_, _ = fmt.Fprintf(os.Stderr, "warning: skipping feature option with unsafe key %q\n", k)
			continue
		}
		envExports += fmt.Sprintf("export %s=%q && ", key, v)
	}

	return fmt.Sprintf(
		`set -e && apt-get update && apt-get install -y curl jq ca-certificates && `+
			`TOKEN=$(curl -s "https://%s/token?service=%s&scope=repository:%s:pull" | jq -r '.token') && `+
			`MANIFEST=$(curl -sL "https://%s/v2/%s/manifests/%s" `+
			`-H "Accept: application/vnd.oci.image.manifest.v1+json" `+
			`-H "Authorization: Bearer $TOKEN") && `+
			`DIGEST=$(echo "$MANIFEST" | jq -r '.layers[0].digest') && `+
			`TMPDIR=$(mktemp -d) && `+
			`curl -sL "https://%s/v2/%s/blobs/$DIGEST" `+
			`-H "Authorization: Bearer $TOKEN" | tar x -C "$TMPDIR" && `+
			`cd "$TMPDIR" && `+
			`%s`+
			`chmod +x install.sh && ./install.sh && `+
			`cd / && rm -rf "$TMPDIR" && `+
			`apt-get clean && rm -rf /var/lib/apt/lists/*`,
		registry, registry, repo,
		registry, repo, tag,
		registry, repo,
		envExports,
	)
}

// parseOCIRef splits an OCI feature reference like "ghcr.io/owner/repo/feature:tag"
// into registry, repository, and tag components.
func parseOCIRef(ref string) (registry, repo, tag string) {
	tag = "latest"
	if idx := strings.LastIndex(ref, ":"); idx != -1 {
		// Only treat as tag if the part after : doesn't contain /
		candidate := ref[idx+1:]
		if !strings.Contains(candidate, "/") {
			tag = candidate
			ref = ref[:idx]
		}
	}

	// First segment is registry, rest is repo
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) == 2 {
		registry = parts[0]
		repo = parts[1]
	} else {
		registry = "ghcr.io"
		repo = ref
	}
	return
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
