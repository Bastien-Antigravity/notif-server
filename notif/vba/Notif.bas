Attribute VB_Name = "Notif"
' -----------------------------------------------------------------------------------------------
' ESSENTIAL PROCESS:
' Raw FFI declarations and bridge bindings connecting VBA to the libnotif Go CGO shared library.
'
' DATA FLOW:
' 1. Input: Target address, severity level, message, attachment, and tags from VBA callers.
' 2. Logic: Marshals calls across the C ABI boundary into libnotif.
' 3. Output: Integer status code (0 = success), error strings, and memory deallocation.
'
' KEY PARAMETERS:
' - LIB_PATH: Path to the compiled libnotif shared library (.dylib or .dll).
' -----------------------------------------------------------------------------------------------

Option Explicit

#If Mac Then
    Private Const LIB_PATH As String = "/usr/local/lib/libnotif.dylib"
#Else
    Private Const LIB_PATH As String = "libnotif.dll"
#End If

' -----------------------------------------------------------------------------------------------
' Core libnotif C ABI Declarations
' -----------------------------------------------------------------------------------------------

Public Declare PtrSafe Function Notif_Send Lib LIB_PATH ( _
    ByVal targetAddr As String, _
    ByVal level As String, _
    ByVal message As String, _
    ByVal attachment As String, _
    ByVal tagsCSV As String) As Long

Public Declare PtrSafe Function Notif_SendWithTimeout Lib LIB_PATH ( _
    ByVal targetAddr As String, _
    ByVal level As String, _
    ByVal message As String, _
    ByVal attachment As String, _
    ByVal tagsCSV As String, _
    ByVal timeoutSec As Double) As Long

Public Declare PtrSafe Function Notif_GetLastError Lib LIB_PATH () As LongPtr
Public Declare PtrSafe Sub Notif_FreeString Lib LIB_PATH (ByVal ptr As LongPtr)
Public Declare PtrSafe Sub Notif_Close Lib LIB_PATH ()
