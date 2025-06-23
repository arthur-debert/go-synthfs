# Golang CLI Project Template

This is a comprehensive template for building Go CLI applications with modern tooling and best practices.

## 🚀 **Recommended: Use the Project Creator**

The easiest way to use this template is via the main repository's project creation script:

```bash
# From the main template repository
./bin/create-project /path/to/your-new-cli-project

# With custom settings
./bin/create-project /path/to/my-tool \
  --description "My awesome CLI tool" \
  --author-name "Your Name"
```

This automatically:

- ✅ Copies and configures the template
- ✅ Replaces all placeholders with your values  
- ✅ Renames directories appropriately
- ✅ Initializes git repository
- ✅ Installs Go dependencies
- ✅ Creates initial commit

## 📋 **Alternative: Manual Setup**

If you prefer manual setup or want to understand the process:

### 1. Template Setup

Replace all placeholder values throughout the project:

```bash
# The project creator automatically handles these replacements:
go-synthfs      → your-cli-name (from directory name)
arthur-debert   → arthur-debert (from config.yaml)
Description of your CLI tool → "Description of your CLI tool" 
Arthur Debert   → "Arthur Debert" (from config.yaml)
arthur@debert.xyz  → "arthur@debert.xyz" (from config.yaml)
```

See `TEMPLATE_USAGE.md` for detailed manual setup instructions.

## ✨ **Features**

### 🏗️ Project Structure

- **`cmd/`** - CLI application main entry points
- **`pkg/`** - Reusable Go packages/libraries
- **`scripts/`** - Development and deployment scripts
- **`.github/workflows/`** - CI/CD workflows

### 🔧 Development Tools

- **Build automation** with comprehensive build scripts
- **Testing** with coverage reporting and gotestsum
- **Linting** with golangci-lint
- **Pre-commit hooks** for code quality
- **Line counting** with cloc for Go projects
- **Release automation** with semantic versioning

### 🚀 Release & Distribution

- **GoReleaser** configuration for multi-platform builds
- **GitHub Actions** for CI/CD
- **Homebrew** formula generation (with debug mode)
- **Debian packages** (.deb) generation
- **Code coverage** reporting with Codecov integration

## 🛠️ **Available Scripts**

### Development Scripts

```bash
# Build the application
./scripts/build

# Run tests with coverage
./scripts/test

# Run tests with detailed coverage report
./scripts/test-with-coverage

# Run linting
./scripts/lint

# Count lines of code
./scripts/cloc-go [directory]

# Install pre-commit hooks
./scripts/pre-commit install

# Create a new release
./scripts/release-new [--major|--minor|--patch] [--yes]
```

### Script Details

#### `scripts/build`

- Builds all Go packages
- Creates CLI binary with version info from git
- Performs basic functionality tests
- Outputs to `bin/` directory

#### `scripts/test`

- Runs all tests with race detection
- Supports `--ci` flag for CI environments
- Generates coverage reports
- Uses gotestsum for better output formatting

#### `scripts/lint`

- Runs golangci-lint with comprehensive checks
- Auto-installs golangci-lint if missing
- Configurable timeout (5 minutes)

#### `scripts/pre-commit`

- Installs/uninstalls Git pre-commit hooks
- Runs linting and testing before commits
- Usage: `./scripts/pre-commit [install|uninstall]`

#### `scripts/release-new`

- Interactive or CLI-driven version bumping
- Supports semantic versioning (major/minor/patch)
- Creates and pushes Git tags
- Triggers GitHub Actions release workflow

#### `scripts/cloc-go`

- Counts lines of code in Go projects
- Separates production code from test code
- Provides detailed statistics
- Usage: `./scripts/cloc-go [directory]` (defaults to `pkg/`)

## 🔄 **GitHub Actions Workflows**

### Test Workflow (`.github/workflows/test.yml`)

- **Build Job**: Compiles packages and CLI binary
- **Test Job**: Runs tests with coverage reporting
- Uploads coverage to Codecov
- Runs on every push and PR

### Release Workflow (`.github/workflows/release.yml`)

- Triggers on version tags (`v*.*.*`)
- Builds multi-platform binaries
- Creates GitHub releases
- Updates Homebrew formula
- Generates Debian packages

## ⚙️ **GoReleaser Configuration**

### Debug Mode for Homebrew

The template supports a debug mode for Homebrew formula generation:

```bash
# Production releases (default)
DEBUG=false → Formula goes to "Formula/" directory

# Debug/testing releases  
DEBUG=true → Formula goes to "debug/" directory
```

### Supported Platforms

- **Operating Systems**: Linux, macOS, Windows
- **Architectures**: amd64, arm64
- **Package Formats**: tar.gz, zip, .deb
- **Distribution**: GitHub Releases, Homebrew

## 🎯 **Configuration**

### Required Secrets (GitHub)

Set these in your GitHub repository settings:

```bash
HOMEBREW_TAP_TOKEN    # GitHub token with access to homebrew-tools repo
CODECOV_TOKEN         # Codecov token for coverage reporting
```

### Optional Environment Variables

```bash
PKG_NAME              # Override package name
DEBUG                 # Enable debug mode for Homebrew formula
```

## 🏆 **Best Practices**

### Development Workflow

1. Install pre-commit hooks: `./scripts/pre-commit install`
2. Write tests for new features
3. Run `./scripts/test` before committing
4. Use `./scripts/release-new` for releases

### Release Process

1. Ensure all changes are committed
2. Run `./scripts/release-new --patch` (or --minor/--major)
3. GitHub Actions will handle the release automatically
4. Monitor the release at your GitHub Actions page

## 📚 **Dependencies**

- **Go 1.23+**
- **golangci-lint** (auto-installed)
- **gotestsum** (auto-installed)
- **cloc** (for line counting)
- **GitHub CLI** (optional, for enhanced GitHub integration)

## 📄 **License**

MIT License - see LICENSE file for details
