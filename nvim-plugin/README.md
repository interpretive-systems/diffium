# Diffium Neovim Plugin

A Neovim plugin that integrates [Diffium](https://github.com/interpretive-systems/diffium), a diff-first TUI for git changes, directly into your editor via a floating terminal window.

## ✨ Features

- Launch Diffium's interactive TUI in a floating window without leaving Neovim
- Default `:Diffium` command to start watching git changes
- Customizable keymap (default: `<C-d>`) for quick access
- Pass arguments to the `diffium` command for advanced usage
- Lightweight Lua-based plugin with minimal dependencies
- Automatic error handling and notifications

## 📋 Prerequisites

- **Neovim** >= 0.5.0 (with Lua support)
- **Diffium** binary installed and available in your `PATH`
- **Git** repository (Diffium requires a git repo to function)

### Installing Diffium

Before using this plugin, you need to install the Diffium CLI tool:

1. Clone the main repository: `git clone https://github.com/interpretive-systems/diffium`
2. Build and install: `cd diffium && go build -o diffium ./cmd/diffium && sudo mv diffium /usr/local/bin/` (or add to your PATH)

For detailed installation instructions, see the [main Diffium README](https://github.com/interpretive-systems/diffium).

## 🧩 Installation

### Manual Installation

Since this plugin is part of the main Diffium repository, install it manually:

1. Clone the Diffium repository: `git clone https://github.com/interpretive-systems/diffium`
2. Copy the plugin files to your Neovim config:
   ```bash
   cp -r diffium/nvim-plugin/lua ~/.config/nvim/lua/
   cp -r diffium/nvim-plugin/plugin ~/.config/nvim/plugin/
   ```
3. Alternatively, create symlinks:
   ```bash
   ln -s $(pwd)/diffium/nvim-plugin/lua ~/.config/nvim/lua/diffium
   ln -s $(pwd)/diffium/nvim-plugin/plugin/diffium.vim ~/.config/nvim/plugin/diffium.vim
   ```
4. Restart Neovim or run `:source ~/.config/nvim/init.lua`

### Plugin Managers

This plugin is not currently published as a separate package. For plugin manager support, you would need to host it as a separate repository. The manual installation above is the recommended approach.

## 🧠 Usage

### Basic Usage

1. Open Neovim in a git repository
2. Run `:Diffium` to launch the Diffium TUI in a floating window
3. Use Diffium's keyboard shortcuts to navigate and interact with your git changes
4. Press `q` in the floating window to close Diffium and return to Neovim

### Command Reference

- `:Diffium` - Launch Diffium with default `watch` command
- `:Diffium <args>` - Launch Diffium with custom arguments (e.g., `:Diffium --repo /path/to/repo`)

### Keybindings

- `<C-d>` - Quick launch Diffium (customizable)

### Diffium TUI Controls

Once launched, use these keys in the floating window:

- `j/k` or arrows: Navigate file list
- `s`: Toggle side-by-side vs inline diff view
- `c`: Open commit flow
- `r`: Refresh changes
- `q`: Quit Diffium

For complete controls, see the [main Diffium documentation](https://github.com/interpretive-systems/diffium).

## ⚙️ Configuration

The plugin can be configured during setup. Add this to your `init.lua`:

```lua
require('diffium').setup({
  command = "Diffium",  -- Custom command name (default: "Diffium")
  keymap = "<C-d>",     -- Custom keymap (default: "<C-d>", set to "" to disable)
})
```

Example with custom settings:

```lua
require('diffium').setup({
  command = "MyDiff",
  keymap = "<leader>d",
})
```

## ❓ FAQ

### Q: "diffium binary not found in PATH" error

**A:** The Diffium CLI tool must be installed and accessible in your system's PATH. Follow these steps:

1. Ensure you have Go installed (`go version`)
2. Clone the Diffium repository: `git clone https://github.com/interpretive-systems/diffium`
3. Build the binary: `cd diffium && go build -o diffium ./cmd/diffium`
4. Move to a directory in your PATH: `sudo mv diffium /usr/local/bin/` (or `~/bin/` if you have it in PATH)
5. Verify: `diffium --help` should work in your terminal
6. Restart Neovim and try again

### Q: The floating window doesn't appear

**A:** Ensure you're in a git repository. Diffium requires a valid git repo to function. Check with `git status` in the directory.

### Q: How do I change the floating window size?

**A:** The window size is calculated as 90% of your Neovim's dimensions. This is currently not configurable but may be added in future versions.

### Q: Can I use this with non-git repositories?

**A:** No, Diffium is specifically designed for git repositories and will not work in non-git directories.

## 🤝 Contributing

Contributions are welcome! Please see the [main Diffium repository](https://github.com/interpretive-systems/diffium) for contribution guidelines.

## 📄 License

This plugin is part of the Diffium project and follows the same license. See [LICENSE](https://github.com/interpretive-systems/diffium/blob/main/LICENSE) in the main repository.
