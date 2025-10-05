-- Neovim plugin integration for Diffium CLI
local M = {}

-- Find diffium binary in PATH
local function find_diffium()
  local path = vim.fn.exepath("diffium")
  if path == "" then
    vim.notify("[Diffium.nvim] diffium binary not found in PATH.", vim.log.levels.ERROR)
    return nil
  end
  return path
end

-- Create floating terminal for Diffium
local function open_floating_term(cmd, opts)
  local buf = vim.api.nvim_create_buf(false, true)
  local width = math.floor(vim.o.columns * 0.9)
  local height = math.floor(vim.o.lines * 0.9)
  local row = math.floor((vim.o.lines - height) / 2)
  local col = math.floor((vim.o.columns - width) / 2)

  local ok, win = pcall(vim.api.nvim_open_win, buf, true, {
    relative = "editor",
    width = width,
    height = height,
    row = row,
    col = col,
    style = "minimal",
    border = "rounded",
  })

  if not ok then
    vim.cmd("terminal " .. cmd)
    vim.notify("[Diffium.nvim] Floating windows not supported, opened in split.", vim.log.levels.INFO)
    return
  end

  vim.fn.termopen(cmd, {
    on_exit = function(_, code)
      vim.schedule(function()
        if code ~= 0 then
          vim.notify(string.format("[Diffium.nvim] exited with code %d", code), vim.log.levels.WARN)
        end
        if opts.auto_close ~= false then
          if vim.api.nvim_buf_is_valid(buf) then
            vim.api.nvim_buf_delete(buf, { force = true })
          end
          if vim.api.nvim_win_is_valid(win) then
            vim.api.nvim_win_close(win, true)
          end
        end
        vim.cmd("stopinsert")
      end)
    end,
  })

  vim.cmd("startinsert")
end

--- Public entrypoint: open Diffium inside Neovim
-- @param args table|nil Optional CLI args for Diffium
function M.open(args)
  local diffium = find_diffium()
  if not diffium then return end

  local cmd = { diffium }
  if args and #args > 0 then
    vim.list_extend(cmd, args)
  else
    table.insert(cmd, "watch")  -- default subcommand
  end

  local joined_cmd = vim.fn.join(cmd, " ")
  open_floating_term(joined_cmd, M.opts or {})
end

--- Setup function for configuration
-- @param opts table { command="Diffium", keymap="<C-d>", auto_close=true }
function M.setup(opts)
  M.opts = opts or {}
  local cmd_name = M.opts.command or "Diffium"
  local keymap = M.opts.keymap or "<C-d>"

  vim.api.nvim_create_user_command(cmd_name, function(params)
    M.open(params.fargs)
  end, { nargs = "*" })

  if keymap and keymap ~= "" then
    vim.keymap.set("n", keymap, string.format(":%s<CR>", cmd_name),
      { noremap = true, silent = true, desc = "Open Diffium" })
  end
end

return M
