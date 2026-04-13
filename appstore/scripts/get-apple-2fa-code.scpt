property timeoutSeconds : 90
property pollIntervalSeconds : 1
property trustDialogTextHints : {"continue in browser", "use this browser", "approve sign in here", "continue sign in"}
property codeEntryDialogTextHints : {"trusted devices", "verification code", "enter the code", "phone verification code", "security code"}

on run
	set deadlineAt to (current date)
	set deadlineAt to deadlineAt + timeoutSeconds

	repeat while (current date) is less than deadlineAt
		set currentWindow to my frontmostFollowUpWindow()
		if currentWindow is not missing value then
			set code to my scanWindowForCode(currentWindow)
			if code is not "" then return code

			set didAdvanceTrustPrompt to my clickTrustButtonIfPresent(currentWindow)
			if didAdvanceTrustPrompt then
				delay pollIntervalSeconds
			else
				delay pollIntervalSeconds
			end if
		else
			delay pollIntervalSeconds
		end if
	end repeat

	error "Timed out waiting for Apple verification code"
end run

on frontmostFollowUpWindow()
	tell application "System Events"
		if not (exists process "FollowUpUI") then return missing value
		tell process "FollowUpUI"
			if not (exists window 1) then return missing value
			return window 1
		end tell
	end tell
end frontmostFollowUpWindow

on clickTrustButtonIfPresent(theWindow)
	if not (my looksLikeTrustDialog(theWindow)) then
		return false
	end if

	return my clickRightmostButton(theWindow)
end clickTrustButtonIfPresent

on looksLikeTrustDialog(theWindow)
	if my windowContainsAnyTextHint(theWindow, codeEntryDialogTextHints) then
		return false
	end if
	return my windowContainsAnyTextHint(theWindow, trustDialogTextHints)
end looksLikeTrustDialog

on windowContainsAnyTextHint(theWindow, hints)
	set sourceText to my textForWindow(theWindow)
	repeat with hintText in hints
		if sourceText contains (contents of hintText) then return true
	end repeat
	return false
end windowContainsAnyTextHint

on scanWindowForCode(currentWindow)
	set sourceText to my textForWindow(currentWindow)
	set extractedCode to do shell script "/bin/printf %s " & quoted form of sourceText & " | /usr/bin/grep -Eo '(^|[^0-9])[0-9]{6}([^0-9]|$)' | /usr/bin/head -n 1 | /usr/bin/tr -cd '0-9'"
	if extractedCode is not "" then
		return extractedCode
	end if

	set extractedCode to my extractSpacedCodeText(sourceText)
	if extractedCode is not "" then
		return extractedCode
	end if

	return ""
end scanWindowForCode

on extractSpacedCodeText(sourceText)
	set extractedCode to do shell script "/bin/printf %s " & quoted form of sourceText & " | /usr/bin/grep -Eo '[0-9]([[:space:][:punct:]]+[0-9]){5}' | /usr/bin/head -n 1"
	if extractedCode is not "" then
		set digitsOnly to my digitsOnlyText(extractedCode)
		if (length of digitsOnly) is 6 then return digitsOnly
	end if

	set digitsOnly to my digitsOnlyText(sourceText)
	if (length of digitsOnly) is 6 then
		return digitsOnly
	end if
	return ""
end extractSpacedCodeText

on digitsOnlyText(sourceText)
	set shellResult to do shell script "/bin/printf %s " & quoted form of sourceText & " | /usr/bin/tr -cd '0-9'"
	return shellResult
end digitsOnlyText

on textForWindow(theWindow)
	tell application "System Events"
		tell process "FollowUpUI"
			set collectedText to {}
			try
				set staticTexts to entire contents of theWindow
				repeat with itemRef in staticTexts
					try
						set end of collectedText to (value of itemRef as text)
					end try
				end repeat
			end try
			return my joinLines(collectedText)
		end tell
	end tell
end textForWindow

on clickRightmostButton(theWindow)
	tell application "System Events"
		tell process "FollowUpUI"
			try
				click button 1 of theWindow
				return true
			on error
				return false
			end try
		end tell
	end tell
end clickRightmostButton

on joinLines(itemsList)
	set AppleScript's text item delimiters to linefeed
	set joinedText to itemsList as text
	set AppleScript's text item delimiters to ""
	return joinedText
end joinLines
