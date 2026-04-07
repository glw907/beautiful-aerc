" aercmail.vim — Syntax highlighting for aerc compose buffers.
"
" This is a custom filetype used instead of the built-in 'mail' filetype,
" which defines many highlight groups that conflict with our color scheme.
" Set via VimEnter autocmd in init.lua.
"
" Highlight colors use the Nord palette. To customize for a different theme,
" change the guifg hex values below. The ctermfg values are 256-color
" approximations for terminals without truecolor support.
"
" Highlight groups:
"   aercmailHeaderKey    — Header field names (From:, To:, Subject:, etc.)
"   aercmailAngleBracket — Email addresses in <angle brackets>
"   aercmailQuote1       — Single-level quoted text (> )
"   aercmailQuote2       — Nested quoted text (> > )
"
" Spell check is excluded from all groups above so the spell checker
" only flags words in the author's own text.

if exists("b:current_syntax")
  finish
endif

" Header keys (To:, From:, Subject:, etc.) — bold blue
syntax match aercmailHeaderKey /^[A-Za-z-]\+:/

" Email addresses in angle brackets — dimmed grey
syntax match aercmailAngleBracket /<[^>]\+>/

" Quoted text — more specific match first so nested quotes override single
syntax match aercmailQuote2 /^> > .*$/
syntax match aercmailQuote2 /^> >$/
syntax match aercmailQuote1 /^> .*$/
syntax match aercmailQuote1 /^>$/

highlight aercmailHeaderKey guifg=#81A1C1 gui=bold ctermfg=110 cterm=bold
highlight aercmailAngleBracket guifg=#616E88 ctermfg=60
highlight aercmailQuote1 guifg=#8FBCBB ctermfg=108
highlight aercmailQuote2 guifg=#616E88 ctermfg=60

" Exclude all custom groups from spell checking
syntax cluster Spell remove=aercmailHeaderKey,aercmailAngleBracket,aercmailQuote1,aercmailQuote2

let b:current_syntax = "aercmail"
