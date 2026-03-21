package config

import "fmt"

// Image represents an official devcontainer template.
type Image struct {
	Name        string // Short name for CLI selection
	Description string
	Reference   string // OCI image reference
	TemplateID  string // OCI template reference (ghcr.io/devcontainers/templates/...)
}

// images is the catalog of official devcontainer templates.
// Source: https://github.com/devcontainers/templates
var images = []Image{
	// Base OS
	{Name: "base", Description: "Ubuntu base image", Reference: "mcr.microsoft.com/devcontainers/base:ubuntu", TemplateID: "ghcr.io/devcontainers/templates/ubuntu"},
	{Name: "alpine", Description: "Minimal Alpine Linux", Reference: "mcr.microsoft.com/devcontainers/base:alpine", TemplateID: "ghcr.io/devcontainers/templates/alpine"},
	{Name: "debian", Description: "Debian Linux", Reference: "mcr.microsoft.com/devcontainers/base:debian", TemplateID: "ghcr.io/devcontainers/templates/debian"},
	{Name: "universal", Description: "Multi-language universal image", Reference: "mcr.microsoft.com/devcontainers/universal:latest", TemplateID: "ghcr.io/devcontainers/templates/universal"},

	// Languages
	{Name: "go", Description: "Go development", Reference: "mcr.microsoft.com/devcontainers/go:latest", TemplateID: "ghcr.io/devcontainers/templates/go"},
	{Name: "node", Description: "Node.js & JavaScript", Reference: "mcr.microsoft.com/devcontainers/javascript-node:latest", TemplateID: "ghcr.io/devcontainers/templates/javascript-node"},
	{Name: "typescript", Description: "Node.js & TypeScript", Reference: "mcr.microsoft.com/devcontainers/typescript-node:latest", TemplateID: "ghcr.io/devcontainers/templates/typescript-node"},
	{Name: "python", Description: "Python 3", Reference: "mcr.microsoft.com/devcontainers/python:latest", TemplateID: "ghcr.io/devcontainers/templates/python"},
	{Name: "rust", Description: "Rust development", Reference: "mcr.microsoft.com/devcontainers/rust:latest", TemplateID: "ghcr.io/devcontainers/templates/rust"},
	{Name: "java", Description: "Java development", Reference: "mcr.microsoft.com/devcontainers/java:latest", TemplateID: "ghcr.io/devcontainers/templates/java"},
	{Name: "dotnet", Description: "C# / .NET", Reference: "mcr.microsoft.com/devcontainers/dotnet:latest", TemplateID: "ghcr.io/devcontainers/templates/dotnet"},
	{Name: "cpp", Description: "C/C++ development", Reference: "mcr.microsoft.com/devcontainers/cpp:latest", TemplateID: "ghcr.io/devcontainers/templates/cpp"},
	{Name: "ruby", Description: "Ruby development", Reference: "mcr.microsoft.com/devcontainers/ruby:latest", TemplateID: "ghcr.io/devcontainers/templates/ruby"},
	{Name: "php", Description: "PHP development", Reference: "mcr.microsoft.com/devcontainers/php:latest", TemplateID: "ghcr.io/devcontainers/templates/php"},

	// Language + Database
	{Name: "go-postgres", Description: "Go with PostgreSQL", Reference: "mcr.microsoft.com/devcontainers/go:latest", TemplateID: "ghcr.io/devcontainers/templates/go-postgres"},
	{Name: "node-postgres", Description: "Node.js with PostgreSQL", Reference: "mcr.microsoft.com/devcontainers/javascript-node:latest", TemplateID: "ghcr.io/devcontainers/templates/javascript-node-postgres"},
	{Name: "node-mongo", Description: "Node.js with MongoDB", Reference: "mcr.microsoft.com/devcontainers/javascript-node:latest", TemplateID: "ghcr.io/devcontainers/templates/javascript-node-mongo"},
	{Name: "python-postgres", Description: "Python 3 with PostgreSQL", Reference: "mcr.microsoft.com/devcontainers/python:latest", TemplateID: "ghcr.io/devcontainers/templates/postgres"},
	{Name: "java-postgres", Description: "Java with PostgreSQL", Reference: "mcr.microsoft.com/devcontainers/java:latest", TemplateID: "ghcr.io/devcontainers/templates/java-postgres"},
	{Name: "rust-postgres", Description: "Rust with PostgreSQL", Reference: "mcr.microsoft.com/devcontainers/rust:latest", TemplateID: "ghcr.io/devcontainers/templates/rust-postgres"},
	{Name: "dotnet-postgres", Description: "C# (.NET) with PostgreSQL", Reference: "mcr.microsoft.com/devcontainers/dotnet:latest", TemplateID: "ghcr.io/devcontainers/templates/dotnet-postgres"},
	{Name: "ruby-rails-postgres", Description: "Ruby on Rails with PostgreSQL", Reference: "mcr.microsoft.com/devcontainers/ruby:latest", TemplateID: "ghcr.io/devcontainers/templates/ruby-rails-postgres"},
	{Name: "php-mariadb", Description: "PHP with MariaDB", Reference: "mcr.microsoft.com/devcontainers/php:latest", TemplateID: "ghcr.io/devcontainers/templates/php-mariadb"},
	{Name: "cpp-mariadb", Description: "C++ with MariaDB", Reference: "mcr.microsoft.com/devcontainers/cpp:latest", TemplateID: "ghcr.io/devcontainers/templates/cpp-mariadb"},
	{Name: "dotnet-mssql", Description: "C# (.NET) with SQL Server", Reference: "mcr.microsoft.com/devcontainers/dotnet:latest", TemplateID: "ghcr.io/devcontainers/templates/dotnet-mssql"},

	// Data Science
	{Name: "anaconda", Description: "Anaconda (Python 3) for data science", Reference: "mcr.microsoft.com/devcontainers/anaconda:latest", TemplateID: "ghcr.io/devcontainers/templates/anaconda"},
	{Name: "miniconda", Description: "Miniconda (Python 3)", Reference: "mcr.microsoft.com/devcontainers/miniconda:latest", TemplateID: "ghcr.io/devcontainers/templates/miniconda"},

	// Docker & Kubernetes
	{Name: "docker-in-docker", Description: "Docker-in-Docker", Reference: "mcr.microsoft.com/devcontainers/base:ubuntu", TemplateID: "ghcr.io/devcontainers/templates/docker-in-docker"},
	{Name: "docker-outside", Description: "Use host Docker daemon", Reference: "mcr.microsoft.com/devcontainers/base:ubuntu", TemplateID: "ghcr.io/devcontainers/templates/docker-outside-of-docker"},
	{Name: "kubernetes", Description: "Kubernetes with Helm", Reference: "mcr.microsoft.com/devcontainers/base:ubuntu", TemplateID: "ghcr.io/devcontainers/templates/kubernetes-helm"},
}

// ListImages returns all known devcontainer templates.
func ListImages() []Image {
	return images
}

// FindImage looks up an image by short name. Returns nil if not found.
func FindImage(name string) *Image {
	for i := range images {
		if images[i].Name == name {
			return &images[i]
		}
	}
	return nil
}

// ImageNames returns the short names for use in help text.
func ImageNames() []string {
	names := make([]string, len(images))
	for i, img := range images {
		names[i] = img.Name
	}
	return names
}

// FormatImageList returns a formatted table of available images.
func FormatImageList() string {
	var s string
	for _, img := range images {
		s += fmt.Sprintf("  %-22s %s\n", img.Name, img.Description)
	}
	return s
}
