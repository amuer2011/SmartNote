Unicode true




!include "MUI2.nsh"




!include "x64.nsh"




!include "FileFunc.nsh"




Var Isolated




Name "SmartNote"




OutFile "${INSTALLER_OUTPUT}"




InstallDir "$LOCALAPPDATA\Programs\SmartNote"




InstallDirRegKey HKCU "Software\SmartNote" "InstallDir"




RequestExecutionLevel user




SetCompressor /SOLID lzma




SetCompressorDictSize 32




VIProductVersion "0.12.18.0"




VIAddVersionKey /LANG=1033 "ProductName" "SmartNote"




VIAddVersionKey /LANG=1033 "FileDescription" "SmartNote Offline Installer"




VIAddVersionKey /LANG=1033 "FileVersion" "0.12.18"




VIAddVersionKey /LANG=1033 "LegalCopyright" "SmartNote"




!define MUI_ICON "${PROJECT_ROOT}\build\windows\icon.ico"




!define MUI_UNICON "${PROJECT_ROOT}\build\windows\icon.ico"




!define MUI_ABORTWARNING




!insertmacro MUI_PAGE_WELCOME




!insertmacro MUI_PAGE_DIRECTORY




!insertmacro MUI_PAGE_INSTFILES




!define MUI_FINISHPAGE_RUN "$INSTDIR\SmartNote.exe"




!insertmacro MUI_PAGE_FINISH




!insertmacro MUI_UNPAGE_CONFIRM




!insertmacro MUI_UNPAGE_INSTFILES




!insertmacro MUI_LANGUAGE "SimpChinese"




!insertmacro MUI_LANGUAGE "English"




Function .onInit




 StrCpy $Isolated "0"




 ${GetParameters} $R0




 ClearErrors




 ${GetOptions} $R0 "/ISOLATED" $R1




 ${IfNot} ${Errors}




  StrCpy $Isolated "1"




 ${EndIf}




 ${IfNot} ${RunningX64}




  MessageBox MB_ICONSTOP "SmartNote requires 64-bit Windows."




  Abort




 ${EndIf}




FunctionEnd




Section "SmartNote" SEC_MAIN




 SetShellVarContext current




 SetOutPath "$INSTDIR"




 File /oname=SmartNote.exe "${APP_BINARY}"




 File /oname=THIRD-PARTY-NOTICES.txt "${PROJECT_ROOT}\build\windows\THIRD-PARTY-NOTICES.txt"




 File /oname=NSIS-LICENSE.txt "${PROJECT_ROOT}\build\windows\NSIS-LICENSE.txt"




 SetOutPath "$INSTDIR\licenses"




 File "${PROJECT_ROOT}\internal\embedding\assets\MODEL-LICENSE"




 File "${PROJECT_ROOT}\internal\embedding\assets\ORT-LICENSE"




 File "${PROJECT_ROOT}\internal\embedding\assets\ORT-ThirdPartyNotices.txt"




 File "${PROJECT_ROOT}\internal\embedding\assets\GO-ORT-LICENSE"




 File "${PROJECT_ROOT}\internal\embedding\assets\VC-LICENSE.html"




 SetOutPath "$INSTDIR"




 WriteUninstaller "$INSTDIR\Uninstall.exe"




 ${If} $Isolated == "1"




  FileOpen $R0 "$INSTDIR\.isolated-test" w




  FileClose $R0




 ${Else}




 CreateDirectory "$SMPROGRAMS\SmartNote"




 CreateShortcut "$SMPROGRAMS\SmartNote\SmartNote.lnk" "$INSTDIR\SmartNote.exe"




 CreateShortcut "$SMPROGRAMS\SmartNote\Uninstall.lnk" "$INSTDIR\Uninstall.exe"




 CreateShortcut "$DESKTOP\SmartNote.lnk" "$INSTDIR\SmartNote.exe"




 WriteRegStr HKCU "Software\SmartNote" "InstallDir" "$INSTDIR"




 WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\SmartNote" "DisplayName" "SmartNote"




 WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\SmartNote" "DisplayVersion" "0.12.18"




 WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\SmartNote" "UninstallString" '$\"$INSTDIR\Uninstall.exe$\"'




 WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\SmartNote" "DisplayIcon" "$INSTDIR\SmartNote.exe"




 WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\SmartNote" "InstallLocation" "$INSTDIR"




 WriteRegDWORD HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\SmartNote" "NoModify" 1




 WriteRegDWORD HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\SmartNote" "NoRepair" 1




 ${EndIf}




SectionEnd




Section "Uninstall"




 SetShellVarContext current




 IfFileExists "$INSTDIR\.isolated-test" skipShortcuts




 Delete "$DESKTOP\SmartNote.lnk"




 Delete "$SMPROGRAMS\SmartNote\SmartNote.lnk"




 Delete "$SMPROGRAMS\SmartNote\Uninstall.lnk"




 RMDir "$SMPROGRAMS\SmartNote"




 skipShortcuts:




 Delete "$INSTDIR\SmartNote.exe"




 Delete "$INSTDIR\THIRD-PARTY-NOTICES.txt"




 Delete "$INSTDIR\NSIS-LICENSE.txt"




 Delete "$INSTDIR\licenses\MODEL-LICENSE"




 Delete "$INSTDIR\licenses\ORT-LICENSE"




 Delete "$INSTDIR\licenses\ORT-ThirdPartyNotices.txt"




 Delete "$INSTDIR\licenses\GO-ORT-LICENSE"




 Delete "$INSTDIR\licenses\VC-LICENSE.html"




 RMDir "$INSTDIR\licenses"




 Delete "$INSTDIR\Uninstall.exe"




 RMDir "$INSTDIR"




 IfFileExists "$INSTDIR\.isolated-test" isolatedCleanup




 DeleteRegKey HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\SmartNote"




 DeleteRegKey HKCU "Software\SmartNote"




 Goto uninstallDone




 isolatedCleanup:




 Delete "$INSTDIR\.isolated-test"




 RMDir "$INSTDIR"




 uninstallDone:




 ; Notes, workspace registrations, attachments and model cache remain intact.




SectionEnd




