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

; On uninstall, stop the sidecars, then OFFER (opt-in) to remove all game data —
; the Docker containers/volumes and the data directory. The default is No, so a
; silent uninstall keeps the user's servers for a possible reinstall.
!macro NSIS_HOOK_PREUNINSTALL
  nsExec::Exec 'taskkill /F /T /IM engine.exe'
  nsExec::Exec 'taskkill /F /IM playit.exe'

  MessageBox MB_YESNO|MB_ICONQUESTION "Also remove all GameHost servers, their saved worlds, and Docker volumes?$\n$\nChoose No to keep them for a future reinstall." /SD IDNO IDNO gh_skip_purge
    nsExec::Exec 'powershell -NoProfile -ExecutionPolicy Bypass -File "$INSTDIR\resources\uninstall-cleanup.ps1"'
  gh_skip_purge:
!macroend
