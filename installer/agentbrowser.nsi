Unicode True
Target amd64-unicode
ManifestDPIAware true

!include MUI2.nsh
!include nsDialogs.nsh
!include LogicLib.nsh
!include FileFunc.nsh
!include StrFunc.nsh
${Using:StrFunc} UnStrStr

!ifndef VERSION
  !define VERSION "0.3.0"
!endif
!ifndef ROOT
  !define ROOT "."
!endif

Name "Agent Browser"
Caption "Agent Browser setup"
OutFile "${ROOT}/dist/AgentBrowserSetup.exe"
InstallDir "$LOCALAPPDATA\Programs\Agent Browser"
InstallDirRegKey HKCU "Software\Agent Browser" "InstallDir"
RequestExecutionLevel user
ShowInstDetails show
BrandingText "Agent Browser"
Icon "${ROOT}/assets/agentbrowser.ico"

!define MUI_ICON "${ROOT}/assets/agentbrowser.ico"
!define MUI_UNICON "${ROOT}/assets/agentbrowser.ico"
!define MUI_ABORTWARNING
!define MUI_WELCOMEPAGE_TITLE "Install Agent Browser"
!define MUI_WELCOMEPAGE_TEXT "Agent Browser keeps a separate Chrome for each person.$\r$\n$\r$\nThis installs it only for you. It does not need an administrator.$\r$\n$\r$\nThe first time you open it, Chrome is downloaded for you.$\r$\n$\r$\nClick Next."
!define MUI_FINISHPAGE_RUN "$INSTDIR\abrowser.exe"
!define MUI_FINISHPAGE_RUN_TEXT "Open Agent Browser"
!define MUI_FINISHPAGE_TITLE "Agent Browser is installed"
!define MUI_FINISHPAGE_TEXT "Shortcuts are on the desktop and in the Start menu. They open the window directly.$\r$\n$\r$\nClick Finish."

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
UninstPage custom un.keepPage un.keepLeave
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "English"

Var KeepCheckbox
Var KeepProfiles

!insertmacro GetSize

Section "Install"
  SetOutPath "$INSTDIR"
  File "/oname=abrowser.exe" "${ROOT}/dist/abrowser.exe"
  File "/oname=agentbrowser.ico" "${ROOT}/assets/agentbrowser.ico"
  WriteUninstaller "$INSTDIR\Uninstall.exe"

  CreateDirectory "$SMPROGRAMS\Agent Browser"
  CreateShortcut "$SMPROGRAMS\Agent Browser\Agent Browser.lnk" "$INSTDIR\abrowser.exe" "" "$INSTDIR\agentbrowser.ico"
  CreateShortcut "$SMPROGRAMS\Agent Browser\Uninstall Agent Browser.lnk" "$INSTDIR\Uninstall.exe" "" "$INSTDIR\agentbrowser.ico"
  CreateShortcut "$DESKTOP\Agent Browser.lnk" "$INSTDIR\abrowser.exe" "" "$INSTDIR\agentbrowser.ico"

  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\AgentBrowser" "DisplayName" "Agent Browser"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\AgentBrowser" "UninstallString" "$\"$INSTDIR\Uninstall.exe$\""
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\AgentBrowser" "QuietUninstallString" "$\"$INSTDIR\Uninstall.exe$\" /S"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\AgentBrowser" "DisplayIcon" "$INSTDIR\agentbrowser.ico"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\AgentBrowser" "DisplayVersion" "${VERSION}"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\AgentBrowser" "Publisher" "Berk"
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\AgentBrowser" "InstallLocation" "$INSTDIR"
  WriteRegDWORD HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\AgentBrowser" "NoModify" 1
  WriteRegDWORD HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\AgentBrowser" "NoRepair" 1
  ${GetSize} "$INSTDIR" "/S=0K" $0 $1 $2
  WriteRegDWORD HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\AgentBrowser" "EstimatedSize" "$0"
SectionEnd

Function un.onInit
  StrCpy $KeepProfiles "1"
  ${GetParameters} $R0
  ${UnStrStr} $R1 $R0 "/DELETEPROFILES"
  StrCmp $R1 "" keep_done
    StrCpy $KeepProfiles "0"
  keep_done:
FunctionEnd

Function un.keepPage
  !insertmacro MUI_HEADER_TEXT "Keep your people" "Their Chromes can stay on this PC."
  nsDialogs::Create 1018
  Pop $0
  ${If} $0 == error
    Abort
  ${EndIf}
  ${NSD_CreateLabel} 0 0 100% 40u "Agent Browser will be removed. Leave this box checked to keep each person's Chrome, mail, and extensions in your personal folder."
  Pop $0
  ${NSD_CreateCheckbox} 0 50u 100% 14u "Keep my people and their browsers"
  Pop $KeepCheckbox
  ${NSD_Check} $KeepCheckbox
  nsDialogs::Show
FunctionEnd

Function un.keepLeave
  ${NSD_GetState} $KeepCheckbox $0
  ${If} $0 <> ${BST_CHECKED}
    StrCpy $KeepProfiles "0"
  ${Else}
    StrCpy $KeepProfiles "1"
  ${EndIf}
FunctionEnd

Section "Uninstall"
  Delete "$INSTDIR\abrowser.exe"
  Delete "$INSTDIR\agentbrowser.ico"
  Delete "$INSTDIR\Uninstall.exe"
  RMDir "$INSTDIR"

  Delete "$DESKTOP\Agent Browser.lnk"
  Delete "$SMPROGRAMS\Agent Browser\Agent Browser.lnk"
  Delete "$SMPROGRAMS\Agent Browser\Uninstall Agent Browser.lnk"
  RMDir "$SMPROGRAMS\Agent Browser"

  DeleteRegKey HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\AgentBrowser"
  DeleteRegKey HKCU "Software\Agent Browser"

  ${If} $KeepProfiles == "0"
    RMDir /r "$LOCALAPPDATA\AgentBrowser"
  ${EndIf}
SectionEnd
