" Initializes Diffium Neovim integration

if exists('g:loaded_diffium_plugin')
  finish
endif
let g:loaded_diffium_plugin = 1

lua << EOF
require('diffium').setup({
  command = "Diffium",
  keymap = "<C-d>",  -- customize if desired
})
EOF
