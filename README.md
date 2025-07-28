# Farm

A modern dotfile manager inspired by GNU Stow with enhanced features for better control and tracking.

## Features

- **Lockfile tracking**: Keeps track of all created symlinks in a lockfile (`farm.lock`)
- **Dead link cleanup**: Automatically removes broken symlinks when re-linking
- **YAML configuration**: Simple and readable YAML configuration
- **Multi-target support**: Symlink a single source to multiple target locations
- **Granular folding control**: Fine-grained control over directory folding behavior
- **Conditional configs**: Enable packages for specific environments (work, home, etc.)


## Installation

You can install Farm by running the install script which will download
the [latest release](https://github.com/mskelton/farm/releases/latest).

```bash
curl -LSfs https://go.mskelton.dev/farm/install | sh
```

Or you can build from source.

```bash
git clone git@github.com:mskelton/farm.git
cd farm
go install ./cmd/farm
```

## Configuration

Create a `farm.yaml` file in your dotfiles directory:

```yaml
packages:
  # Package that's always enabled (no environments specified)
  - source: ./vim
    targets:
      - ~/.vim
      - ~/.config/nvim

  # Package only enabled for work environment
  - source: ./vscode
    targets:
      - ~/Library/Application Support/Code/User
      - ~/Library/Application Support/Cursor/User
    environments:
      - work

  # Package only enabled for home environment
  - source: ./personal-config
    targets:
      - ~/.config
    environments:
      - home

  # Package enabled for both work and home
  - source: ./config
    targets:
      - ~/.config
    default_fold: false  # Don't fold directories by default
    fold:
      - bin              # But do fold the bin directory
    no_fold:
      - sensitive        # Never fold the sensitive directory
    environments:
      - work
      - home
```

## Usage

### Create symlinks

```bash
# Link all packages (default behavior)
farm link

# Link only work-specific packages
farm link work

# Link only home-specific packages
farm link home
```

### Remove symlinks

```bash
# Remove all symlinks
farm unlink

# Remove only work-specific symlinks
farm unlink work

# Remove only home-specific symlinks
farm unlink home
```

### Check status

```bash
# Check status of all symlinks
farm status

# Check status of work environment
farm status work

# Check status of home environment
farm status home
```

### Dry run (see what would be done)

```bash
# Dry run for all packages
farm link --dry-run

# Dry run for work environment
farm link work --dry-run
```

### Verbose output

```bash
# Verbose output for all packages
farm link -v

# Verbose output for work environment
farm link work -v
```

## Conditional Configs

Farm supports conditional configuration through environments. This allows you to:

- **Organize by context**: Keep work and personal configurations separate
- **Selective linking**: Only link packages relevant to your current environment
- **Clean separation**: Maintain different dotfile sets for different machines or contexts

### Environment Behavior

- **No environments specified**: Package is always enabled (backward compatible)
- **Single environment**: Package only enabled for that specific environment
- **Multiple environments**: Package enabled for all specified environments

### Example Workflows

**Work Environment:**
```bash
# Link only work-related packages
farm link work

# Check what's linked for work
farm status work
```

**Home Environment:**
```bash
# Link only home-related packages
farm link home

# Check what's linked for home
farm status home
```

**Mixed Environment:**
```bash
# Link packages that work in both environments
farm link work home

# Note: This will link packages that have either 'work' OR 'home' in their environments list
```

## Folding Behavior

By default, Farm creates individual symlinks for each file (no-folding). You can control this behavior:

- `default_fold: true`: Fold directories by default (symlink entire directories)
- `fold: [list]`: Always fold these directories/patterns
- `no_fold: [list]`: Never fold these directories/patterns

The `no_fold` list takes precedence over `fold` and `default_fold`.

## Lockfile

The lockfile (`farm.lock`) tracks all created symlinks and is used to:
- Clean up dead symlinks when source files are moved or deleted
- Show the status of all managed symlinks

## Example Workflow

1. Set up your dotfiles repository:

```bash
mkdir ~/dotfiles
cd ~/dotfiles
```

2. Organize your dotfiles by package and environment:

```bash
mkdir -p vim vscode-work vscode-home zsh
mv ~/.vimrc vim/
mv ~/.config/Code/User/settings.json vscode-work/
mv ~/.config/Cursor/User/settings.json vscode-home/
mv ~/.zshrc zsh/
```

3. Create `farm.yaml`:

```yaml
packages:
  - source: ./vim
    targets:
      - '~'

  - source: ./vscode-work
    targets:
      - ~/.config/Code/User
    environments:
      - work

  - source: ./vscode-home
    targets:
      - ~/.config/Cursor/User
    environments:
      - home

  - source: ./zsh
    targets:
      - '~'
```

4. Link your dotfiles for the appropriate environment:

```bash
# For work
farm link work -v

# For home
farm link home -v
```

5. Check status:

```bash
farm status work
farm status home
```

## Testing

Run the comprehensive test suite:

```bash
go test ./... -v
```
