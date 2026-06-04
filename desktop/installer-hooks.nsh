; Custom NSIS hooks for the GameHost installer.
;
; The engine and playit relay run as background "sidecar" processes. When an
; update is installed while GameHost is running, those processes keep
; engine.exe / playit.exe open and the installer fails with
; "Error opening file for writing". Killing them before files are copied
; releases the locks so updates install cleanly.
!macro NSIS_HOOK_PREINSTALL
  nsExec::Exec 'taskkill /F /T /IM engine.exe'
  nsExec::Exec 'taskkill /F /IM playit.exe'
!macroend
