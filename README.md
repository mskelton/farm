# Farm

A modern dotfile manager inspired by GNU Stow with enhanced features for better control and tracking.

## Features

- **Lockfile tracking**: Keeps track of all created symlinks in a lockfile (`farm.lock`)
- **Dead link cleanup**: Automatically removes broken symlinks when re-linking
- **YAML configuration**: Simple and readable YAML configuration instead of `.stowrc`
- **Multi-target support**: Symlink a single source to multiple target locations
- **Granular folding control**: Fine-grained control over directory folding behavior

## Installation

```bash
go build -o farm ./cmd/farm
```

## Configuration

Create a `farm.yaml` file in your dotfiles directory:

```yaml
packages:
  vim:
    source: ./vim
    targets:
      - ~/.vim
      - ~/.config/nvim

  vscode:
    source: ./vscode
    targets:
      - ~/.config/Code/User
      - ~/.config/Cursor/User

  config:
    source: ./config
    targets:
      - ~/.config
    default_fold: false  # Don't fold directories by default
    fold:
      - bin              # But do fold the bin directory
    no_fold:
      - sensitive        # Never fold the sensitive directory
```

## Usage

### Link all packages
```bash
farm link
```

### Link specific package
```bash
farm link vim
```

### Remove symlinks for a package
```bash
farm unlink vim
```

### Check status
```bash
farm status
```

### Dry run (see what would be done)
```bash
farm link --dry-run
```

### Verbose output
```bash
farm link -v
```

## Folding Behavior

By default, `farm` creates individual symlinks for each file (no-folding). You can control this behavior:

- `default_fold: true`: Fold directories by default (symlink entire directories)
- `fold: [list]`: Always fold these directories/patterns
- `no_fold: [list]`: Never fold these directories/patterns

The `no_fold` list takes precedence over `fold` and `default_fold`.

## Lockfile

The lockfile (`farm.lock`) tracks all created symlinks and is used to:
- Clean up dead symlinks when source files are moved or deleted
- Track which package owns each symlink
- Show the status of all managed symlinks

## Example Workflow

1. Set up your dotfiles repository:
```bash
mkdir ~/dotfiles
cd ~/dotfiles
```

2. Organize your dotfiles by package:
```bash
mkdir -p vim vscode zsh
mv ~/.vimrc vim/
mv ~/.config/Code/User/settings.json vscode/
mv ~/.zshrc zsh/
```

3. Create `farm.yaml`:
```yaml
packages:
  vim:
    source: ./vim
    targets:
      - ~
  vscode:
    source: ./vscode
    targets:
      - ~/.config/Code/User
  zsh:
    source: ./zsh
    targets:
      - ~
```

4. Link your dotfiles:
```bash
farm link -v
```

5. Check status:
```bash
farm status
```

## Testing

Run the comprehensive test suite:
```bash
go test ./... -v
```
