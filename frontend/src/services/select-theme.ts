// Shared by the assistant's model picker, scope and reasoning controls, including
// menus teleported outside the sidebar or settings window.
export const selectTheme = {
 peers: {
  InternalSelection: {
   color:'var(--bg)',colorActive:'var(--bg)',colorDisabled:'var(--soft)',
   textColor:'var(--text)',placeholderColor:'var(--muted)',
   border:'1px solid var(--line)',borderHover:'1px solid var(--accent)',
   borderActive:'1px solid var(--accent)',borderFocus:'1px solid var(--accent)',
   boxShadowActive:'0 0 0 2px var(--accent-bg)',boxShadowFocus:'0 0 0 2px var(--accent-bg)',
   borderRadius:'6px'
  },
  InternalSelectMenu: {
   color:'var(--bg)',optionTextColor:'var(--text)',optionTextColorActive:'var(--accent)',
   optionColorActive:'var(--accent-bg)',optionColorPending:'var(--soft)',
   optionColorActivePending:'var(--accent-bg)',groupHeaderTextColor:'var(--muted)',
   borderRadius:'8px'
  }
 }
}
