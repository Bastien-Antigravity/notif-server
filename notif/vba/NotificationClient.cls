VERSION 1.0 CLASS
BEGIN
  MultiUse = -1  'True
END
Attribute VB_Name = "NotificationClient"
Attribute VB_GlobalNameSpace = False
Attribute VB_Creatable = True
Attribute VB_PredeclaredId = False
Attribute VB_Exposed = True
' -----------------------------------------------------------------------------------------------
' ESSENTIAL PROCESS:
' Direct Notification Client class enabling VBA applications to dispatch direct
' alerts to notif-server via the libnotif CGO shared library.
'
' DATA FLOW:
' 1. Input: Alert message, level, tags, or target address.
' 2. Logic: Calls Notif.Notif_SendWithTimeout, retrieves errors via Notif_GetLastError.
' 3. Output: Boolean success status and error diagnostics.
'
' KEY PARAMETERS:
' - m_targetAddr: Resolved host:port string of notif-server (gRPC port 1027).
' - m_timeoutSec: Timeout in seconds for RPC dispatch (default: 5.0).
' -----------------------------------------------------------------------------------------------

Option Explicit

Private m_targetAddr As String
Private m_timeoutSec As Double
Private m_closed As Boolean
Private m_lastError As String

#If Mac Then
    Private Declare PtrSafe Function strlen Lib "/usr/lib/libc.dylib" (ByVal ptr As LongPtr) As Long
    Private Declare PtrSafe Function strlcpy Lib "/usr/lib/libc.dylib" (ByVal dest As String, ByVal src As LongPtr, ByVal size As Long) As Long
#Else
    Private Declare PtrSafe Function lstrlenA Lib "kernel32" (ByVal ptr As LongPtr) As Long
    Private Declare PtrSafe Function lstrcpynA Lib "kernel32" (ByVal dest As String, ByVal src As LongPtr, ByVal size As Long) As Long
#End If

Private Sub Class_Initialize()
    m_targetAddr = ""
    m_timeoutSec = 5#
    m_closed = False
    m_lastError = ""
End Sub

Private Sub Class_Terminate()
    m_closed = True
End Sub

Public Sub Init(ByVal targetAddr As String)
    If Trim$(targetAddr) = "" Then
        m_lastError = "targetAddr cannot be empty"
        Err.Raise vbObjectError + 1001, "NotificationClient", m_lastError
    End If
    m_targetAddr = Trim$(targetAddr)
    m_closed = False
End Sub

Public Property Get TargetAddr() As String
    TargetAddr = m_targetAddr
End Property

Public Property Get LastError() As String
    LastError = m_lastError
End Property

Public Property Get IsClosed() As Boolean
    IsClosed = m_closed
End Property

Public Sub SetTimeout(ByVal seconds As Double)
    If seconds > 0# Then
        m_timeoutSec = seconds
    End If
End Sub

Public Function Send(ByVal message As String, Optional ByVal tagsCSV As String = "") As Boolean
    Send = SendWithAttachment("", message, "", tagsCSV)
End Function

Public Function SendAlert(ByVal level As String, ByVal message As String, Optional ByVal tagsCSV As String = "") As Boolean
    SendAlert = SendWithAttachment(level, message, "", tagsCSV)
End Function

Public Function SendWithAttachment(ByVal level As String, ByVal message As String, ByVal attachment As String, Optional ByVal tagsCSV As String = "") As Boolean
    If m_closed Then
        m_lastError = "NotificationClient is closed"
        SendWithAttachment = False
        Exit Function
    End If

    If m_targetAddr = "" Then
        m_lastError = "NotificationClient target address is empty"
        SendWithAttachment = False
        Exit Function
    End If

    Dim rc As Long
    rc = Notif.Notif_SendWithTimeout(m_targetAddr, level, message, attachment, tagsCSV, m_timeoutSec)

    If rc <> 0 Then
        Dim errPtr As LongPtr
        errPtr = Notif.Notif_GetLastError()
        If errPtr <> 0 Then
            m_lastError = PtrToString(errPtr)
            Notif.Notif_FreeString errPtr
        Else
            m_lastError = "Unknown RPC error (rc=" & CStr(rc) & ")"
        End If
        SendWithAttachment = False
    Else
        m_lastError = ""
        SendWithAttachment = True
    End If
End Function

Public Sub CloseClient()
    m_closed = True
End Sub

Private Function PtrToString(ByVal ptr As LongPtr) As String
    If ptr = 0 Then
        PtrToString = ""
        Exit Function
    End If
    Dim length As Long
    Dim res As String
    #If Mac Then
        length = strlen(ptr)
        res = Space(length)
        strlcpy res, ptr, length + 1
    #Else
        length = lstrlenA(ptr)
        res = Space(length)
        lstrcpynA res, ptr, length + 1
    #End If
    PtrToString = res
End Function
