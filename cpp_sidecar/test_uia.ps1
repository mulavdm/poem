param(
    [string]$GalleryPath = (Join-Path $PSScriptRoot '..\dist\portable\POEM_gallery_uia.exe')
)

$ErrorActionPreference = 'Stop'
$gallery = (Resolve-Path -LiteralPath $GalleryPath).Path
$process = Start-Process -FilePath $gallery -PassThru

try {
    Add-Type -AssemblyName UIAutomationClient
    Add-Type -AssemblyName UIAutomationTypes
    if (-not ('PoemUiaEventCounter' -as [type])) {
        $eventHandlerType = ([System.Windows.Automation.Automation].GetMethods() |
            Where-Object Name -eq 'AddAutomationPropertyChangedEventHandler').GetParameters()[2].ParameterType
        $references = @([System.Windows.Automation.Automation].Assembly.Location, $eventHandlerType.Assembly.Location)
        Add-Type -ReferencedAssemblies $references -TypeDefinition @'
using System.Threading;
using System.Windows.Automation;
public static class PoemUiaEventCounter {
    public static int Count;
    public static int TextCount;
    public static readonly AutomationPropertyChangedEventHandler Handler = OnChanged;
    public static readonly AutomationEventHandler TextHandler = OnTextEvent;
    private static void OnChanged(object sender, AutomationPropertyChangedEventArgs args) {
        Interlocked.Increment(ref Count);
    }
    private static void OnTextEvent(object sender, AutomationEventArgs args) {
        Interlocked.Increment(ref TextCount);
    }
}
'@
    }
    $root = $null
    $deadline = [DateTime]::UtcNow.AddSeconds(10)
    while ($null -eq $root -and [DateTime]::UtcNow -lt $deadline) {
        Start-Sleep -Milliseconds 200
        $condition = [System.Windows.Automation.PropertyCondition]::new(
            [System.Windows.Automation.AutomationElement]::AutomationIdProperty,
            'poem.application'
        )
        $root = [System.Windows.Automation.AutomationElement]::RootElement.FindFirst(
            [System.Windows.Automation.TreeScope]::Descendants,
            $condition
        )
    }
    if ($null -eq $root) { throw 'POEM semantic root was not published within 10 seconds' }

    function Find-POEMElement([string]$id) {
        $condition = [System.Windows.Automation.PropertyCondition]::new(
            [System.Windows.Automation.AutomationElement]::AutomationIdProperty,
            $id
        )
        $element = $root.FindFirst([System.Windows.Automation.TreeScope]::Descendants, $condition)
        if ($null -eq $element) { throw "UIA element '$id' was not found" }
        return $element
    }

    $slider = Find-POEMElement 'gallery.quality'
    $range = [System.Windows.Automation.RangeValuePattern]$slider.GetCurrentPattern(
        [System.Windows.Automation.RangeValuePattern]::Pattern
    )
    if ($range.Current.Minimum -ne 0 -or $range.Current.Maximum -ne 100 -or $range.Current.SmallChange -ne 5) {
        throw "unexpected slider range: $($range.Current.Minimum)..$($range.Current.Maximum), step $($range.Current.SmallChange)"
    }
    $range.SetValue(80)

    $checkbox = Find-POEMElement 'gallery.notifications'
    $toggle = [System.Windows.Automation.TogglePattern]$checkbox.GetCurrentPattern(
        [System.Windows.Automation.TogglePattern]::Pattern
    )
    $before = $toggle.Current.ToggleState
    [PoemUiaEventCounter]::Count = 0
    [System.Windows.Automation.Automation]::AddAutomationPropertyChangedEventHandler(
        $root,
        [System.Windows.Automation.TreeScope]::Subtree,
        [PoemUiaEventCounter]::Handler,
        [System.Windows.Automation.TogglePattern]::ToggleStateProperty
    )
    try {
        $toggle.Toggle()
        Start-Sleep -Milliseconds 500
    } finally {
        [System.Windows.Automation.Automation]::RemoveAutomationPropertyChangedEventHandler(
            $root,
            [PoemUiaEventCounter]::Handler
        )
    }

    $rangeAfter = [System.Windows.Automation.RangeValuePattern](Find-POEMElement 'gallery.quality').GetCurrentPattern(
        [System.Windows.Automation.RangeValuePattern]::Pattern
    )
    $toggleAfter = [System.Windows.Automation.TogglePattern](Find-POEMElement 'gallery.notifications').GetCurrentPattern(
        [System.Windows.Automation.TogglePattern]::Pattern
    )
    if ($rangeAfter.Current.Value -ne 80) { throw "slider action did not reach Go; value is $($rangeAfter.Current.Value)" }
    if ($toggleAfter.Current.ToggleState -eq $before) { throw 'toggle action did not reach Go' }
    if ([PoemUiaEventCounter]::Count -lt 1) { throw 'UIA toggle property-change notification was not raised' }
	$sliderVerifiedValue = $rangeAfter.Current.Value
	$toggleVerifiedState = $toggleAfter.Current.ToggleState.ToString()
	$primary = Find-POEMElement 'gallery.primary'
	if ($primary.Current.AccessKey -ne 'Alt+P') {
		throw "button access key was '$($primary.Current.AccessKey)'"
	}
	$selectorFocus = Invoke-RestMethod -Method Post -Uri 'http://127.0.0.1:47831/focus' -ContentType 'application/json' -Body '{"selector":{"role":"button","name":"Primary","states":{"enabled":true}}}'
	Start-Sleep -Milliseconds 250
	$primary = Find-POEMElement 'gallery.primary'
	if ($selectorFocus.target_id -ne 'gallery.primary' -or -not $primary.Current.HasKeyboardFocus) {
		throw "semantic selector resolved '$($selectorFocus.target_id)' without focusing Primary"
	}
	$semanticSelectorTarget = $selectorFocus.target_id
	Invoke-RestMethod -Method Post -Uri 'http://127.0.0.1:47831/press-key' -ContentType 'application/json' -Body '{"key":"Ctrl+Shift+Q"}' | Out-Null
	Start-Sleep -Milliseconds 300
	$shortcutRange = [System.Windows.Automation.RangeValuePattern](Find-POEMElement 'gallery.quality').GetCurrentPattern(
		[System.Windows.Automation.RangeValuePattern]::Pattern
	)
	if ($shortcutRange.Current.Value -ne 77) {
		throw "generic shortcut did not update quality; value is $($shortcutRange.Current.Value)"
	}
	$shortcutVerifiedValue = $shortcutRange.Current.Value
	Invoke-RestMethod -Method Post -Uri 'http://127.0.0.1:47831/press-key' -ContentType 'application/json' -Body '{"key":"Alt+P"}' | Out-Null
	Start-Sleep -Milliseconds 300
	$primary = Find-POEMElement 'gallery.primary'
	if (-not $primary.Current.HasKeyboardFocus) {
		throw 'button mnemonic did not focus its target'
	}
	$mnemonicAccessKey = $primary.Current.AccessKey

	$inputsTab = Find-POEMElement 'gallery.tabs/inputs'
    $selectionItem = [System.Windows.Automation.SelectionItemPattern]$inputsTab.GetCurrentPattern(
        [System.Windows.Automation.SelectionItemPattern]::Pattern
    )
	$selectionItem.Select()
	Start-Sleep -Milliseconds 500
	$nameInput = Find-POEMElement 'gallery.name'
	$labeledBy = $nameInput.GetCurrentPropertyValue([System.Windows.Automation.AutomationElement]::LabeledByProperty)
	if ($null -eq $labeledBy -or $labeledBy.Current.AutomationId -ne 'gallery.name-field/label') {
		throw 'visible field label was not exposed through UIA LabeledBy'
	}
	$relationshipTargets = $labeledBy.Current.AutomationId
	$formatSelect = Find-POEMElement 'gallery.format'
	$formatValue = [System.Windows.Automation.ValuePattern]$formatSelect.GetCurrentPattern(
		[System.Windows.Automation.ValuePattern]::Pattern
	)
	$formatValue.SetValue('webp')
	Start-Sleep -Milliseconds 300
	$formatSelect = Find-POEMElement 'gallery.format'
	$formatValue = [System.Windows.Automation.ValuePattern]$formatSelect.GetCurrentPattern(
		[System.Windows.Automation.ValuePattern]::Pattern
	)
	if ($formatValue.Current.Value -ne 'WebP image') {
		throw "combo-box ValuePattern selected '$($formatValue.Current.Value)'"
	}
	$formatSelect.SetFocus()
	$formatExpand = [System.Windows.Automation.ExpandCollapsePattern]$formatSelect.GetCurrentPattern(
		[System.Windows.Automation.ExpandCollapsePattern]::Pattern
	)
	$formatExpand.Expand()
	Start-Sleep -Milliseconds 300
	$webpOption = Find-POEMElement 'gallery.format.options/webp'
	if (-not $webpOption.Current.HasKeyboardFocus) {
		throw 'expanded combo box did not expose its active option as focused'
	}
	Invoke-RestMethod -Method Post -Uri 'http://127.0.0.1:47831/press-key' -ContentType 'application/json' -Body '{"key":"Down"}' | Out-Null
	Start-Sleep -Milliseconds 250
	$pngOption = Find-POEMElement 'gallery.format.options/png'
	if (-not $pngOption.Current.HasKeyboardFocus) {
		throw 'combo-box Down navigation did not wrap to PNG'
	}
	Invoke-RestMethod -Method Post -Uri 'http://127.0.0.1:47831/press-key' -ContentType 'application/json' -Body '{"key":"Enter"}' | Out-Null
	Start-Sleep -Milliseconds 300
	$formatSelect = Find-POEMElement 'gallery.format'
	$formatValue = [System.Windows.Automation.ValuePattern]$formatSelect.GetCurrentPattern(
		[System.Windows.Automation.ValuePattern]::Pattern
	)
	$comboKeyboardValue = $formatValue.Current.Value
	if ($comboKeyboardValue -ne 'PNG image' -or -not $formatSelect.Current.HasKeyboardFocus) {
		throw "combo-box keyboard commit produced '$comboKeyboardValue' or lost owner focus"
	}
	$datePicker = Find-POEMElement 'gallery.due-date'
	$datePicker.SetFocus()
	$dateExpand = [System.Windows.Automation.ExpandCollapsePattern]$datePicker.GetCurrentPattern(
		[System.Windows.Automation.ExpandCollapsePattern]::Pattern
	)
	$dateExpand.Expand()
	Start-Sleep -Milliseconds 300
	$calendar = Find-POEMElement 'gallery.due-date.calendar'
	$calendarGrid = [System.Windows.Automation.GridPattern]$calendar.GetCurrentPattern(
		[System.Windows.Automation.GridPattern]::Pattern
	)
	if ($calendarGrid.Current.RowCount -ne 6 -or $calendarGrid.Current.ColumnCount -ne 7) {
		throw "calendar grid dimensions are $($calendarGrid.Current.RowCount)x$($calendarGrid.Current.ColumnCount)"
	}
	$calendarGridShape = "$($calendarGrid.Current.RowCount)x$($calendarGrid.Current.ColumnCount)"
	$activeDate = $calendarGrid.GetItem(3, 0)
	if ($activeDate.Current.AutomationId -ne 'gallery.due-date.calendar/date/2026-06-21' -or -not $activeDate.Current.HasKeyboardFocus) {
		throw "calendar active date is '$($activeDate.Current.AutomationId)' or lacks focus"
	}
	Invoke-RestMethod -Method Post -Uri 'http://127.0.0.1:47831/press-key' -ContentType 'application/json' -Body '{"key":"Page Down"}' | Out-Null
	Start-Sleep -Milliseconds 250
	$julyDate = Find-POEMElement 'gallery.due-date.calendar/date/2026-07-21'
	if (-not $julyDate.Current.HasKeyboardFocus) {
		throw 'calendar Page Down did not retain the day in the next month'
	}
	Invoke-RestMethod -Method Post -Uri 'http://127.0.0.1:47831/press-key' -ContentType 'application/json' -Body '{"key":"Home"}' | Out-Null
	Invoke-RestMethod -Method Post -Uri 'http://127.0.0.1:47831/press-key' -ContentType 'application/json' -Body '{"key":"Enter"}' | Out-Null
	Start-Sleep -Milliseconds 300
	$datePicker = Find-POEMElement 'gallery.due-date'
	$dateValue = [System.Windows.Automation.ValuePattern]$datePicker.GetCurrentPattern(
		[System.Windows.Automation.ValuePattern]::Pattern
	)
	$calendarKeyboardValue = $dateValue.Current.Value
	if ($calendarKeyboardValue -ne '2026-07-19' -or -not $datePicker.Current.HasKeyboardFocus) {
		throw "calendar keyboard commit produced '$calendarKeyboardValue' or lost owner focus"
	}
	$language = Find-POEMElement 'gallery.language'
	$language.SetFocus()
	$languageExpand = [System.Windows.Automation.ExpandCollapsePattern]$language.GetCurrentPattern(
		[System.Windows.Automation.ExpandCollapsePattern]::Pattern
	)
	$languageExpand.Expand()
	Start-Sleep -Milliseconds 250
	$null = Find-POEMElement 'gallery.language.suggestions'
	Invoke-RestMethod -Method Post -Uri 'http://127.0.0.1:47831/press-key' -ContentType 'application/json' -Body '{"key":"Tab"}' | Out-Null
	Start-Sleep -Milliseconds 250
	$suggestionsCondition = [System.Windows.Automation.PropertyCondition]::new(
		[System.Windows.Automation.AutomationElement]::AutomationIdProperty,
		'gallery.language.suggestions'
	)
	if ($null -ne $root.FindFirst([System.Windows.Automation.TreeScope]::Descendants, $suggestionsCondition)) {
		throw 'Tab left the anchored autocomplete suggestions open'
	}
	$datePicker = Find-POEMElement 'gallery.due-date'
	if (-not $datePicker.Current.HasKeyboardFocus) {
		throw 'Tab did not advance from autocomplete to the next control'
	}
	$anchoredOverlayDismissal = 'gallery.language.suggestions'
	$formatSelect = Find-POEMElement 'gallery.format'
	$formatSelect.SetFocus()
	$formatExpand = [System.Windows.Automation.ExpandCollapsePattern]$formatSelect.GetCurrentPattern(
		[System.Windows.Automation.ExpandCollapsePattern]::Pattern
	)
	$formatExpand.Expand()
	Start-Sleep -Milliseconds 200
	$null = Find-POEMElement 'gallery.format.options'
	Invoke-RestMethod -Method Post -Uri 'http://127.0.0.1:47831/click' -ContentType 'application/json' -Body '{"id":"gallery.format"}' | Out-Null
	Start-Sleep -Milliseconds 200
	$formatOptionsCondition = [System.Windows.Automation.PropertyCondition]::new(
		[System.Windows.Automation.AutomationElement]::AutomationIdProperty,
		'gallery.format.options'
	)
	if ($null -ne $root.FindFirst([System.Windows.Automation.TreeScope]::Descendants, $formatOptionsCondition)) {
		throw 'select owner click dismissed then immediately reopened its popup'
	}
	$formatSelect = Find-POEMElement 'gallery.format'
	$formatExpand = [System.Windows.Automation.ExpandCollapsePattern]$formatSelect.GetCurrentPattern(
		[System.Windows.Automation.ExpandCollapsePattern]::Pattern
	)
	$formatExpand.Expand()
	Start-Sleep -Milliseconds 200
	Invoke-RestMethod -Method Post -Uri 'http://127.0.0.1:47831/click' -ContentType 'application/json' -Body '{"id":"gallery.name"}' | Out-Null
	Start-Sleep -Milliseconds 200
	if ($null -ne $root.FindFirst([System.Windows.Automation.TreeScope]::Descendants, $formatOptionsCondition)) {
		throw 'outside click left the anchored select popup open'
	}
	$nameAfterOutsideClick = Find-POEMElement 'gallery.name'
	if (-not $nameAfterOutsideClick.Current.HasKeyboardFocus) {
		throw 'outside click did not transfer focus to its target'
	}
	$outsideClickTarget = $nameAfterOutsideClick.Current.AutomationId
    $textField = Find-POEMElement 'gallery.name'
	[PoemUiaEventCounter]::TextCount = 0
	[System.Windows.Automation.Automation]::AddAutomationEventHandler(
		[System.Windows.Automation.TextPattern]::TextChangedEvent,
		$textField,
		[System.Windows.Automation.TreeScope]::Element,
		[PoemUiaEventCounter]::TextHandler
	)
	[System.Windows.Automation.Automation]::AddAutomationEventHandler(
		[System.Windows.Automation.TextPattern]::TextSelectionChangedEvent,
		$textField,
		[System.Windows.Automation.TreeScope]::Element,
		[PoemUiaEventCounter]::TextHandler
	)
    $valuePattern = [System.Windows.Automation.ValuePattern]$textField.GetCurrentPattern(
        [System.Windows.Automation.ValuePattern]::Pattern
    )
    $unicodeText = 'A' + [char]::ConvertFromUtf32(0x1F600) + [char]0x65E5 + [char]0x672C + 'Z'
    $expectedSelection = [char]::ConvertFromUtf32(0x1F600) + [char]0x65E5
    $valuePattern.SetValue($unicodeText)
	$documentText = $null
	$textDeadline = [DateTime]::UtcNow.AddSeconds(3)
	while ($documentText -ne $unicodeText -and [DateTime]::UtcNow -lt $textDeadline) {
		Start-Sleep -Milliseconds 100
		try {
			$textField = Find-POEMElement 'gallery.name'
			$textPattern = [System.Windows.Automation.TextPattern]$textField.GetCurrentPattern(
				[System.Windows.Automation.TextPattern]::Pattern
			)
			$documentText = $textPattern.DocumentRange.GetText(-1)
		} catch {
			$documentText = $null
		}
	}
	if ($documentText -ne $unicodeText) { throw 'TextPattern did not expose the Unicode document within 3 seconds' }
    $textRange = $textPattern.DocumentRange.Clone()
    $textRange.MoveEndpointByRange(
        [System.Windows.Automation.Text.TextPatternRangeEndpoint]::End,
        $textRange,
        [System.Windows.Automation.Text.TextPatternRangeEndpoint]::Start
    )
    [void]$textRange.MoveEndpointByUnit(
        [System.Windows.Automation.Text.TextPatternRangeEndpoint]::Start,
        [System.Windows.Automation.Text.TextUnit]::Character,
        1
    )
    [void]$textRange.MoveEndpointByUnit(
        [System.Windows.Automation.Text.TextPatternRangeEndpoint]::End,
        [System.Windows.Automation.Text.TextUnit]::Character,
        2
    )
    $textRange.Select()
    Start-Sleep -Milliseconds 500
    $textField = Find-POEMElement 'gallery.name'
    $selectedText = ([System.Windows.Automation.TextPattern]$textField.GetCurrentPattern(
        [System.Windows.Automation.TextPattern]::Pattern
    )).GetSelection()[0].GetText(-1)
    if ($selectedText -ne $expectedSelection) { throw "UTF-16 text range returned an unexpected selection" }
    $automation = Invoke-RestMethod -Uri 'http://127.0.0.1:47831/components'
    $automationNode = $automation.flat | Where-Object id -eq 'gallery.name'
    if ($automationNode.selection_start -ne 1 -or $automationNode.selection_end -ne 3) {
        throw "native UTF-16 selection did not map to rune range 1:3"
    }
	[System.Windows.Automation.Automation]::RemoveAutomationEventHandler(
		[System.Windows.Automation.TextPattern]::TextChangedEvent,
		$textField,
		[PoemUiaEventCounter]::TextHandler
	)
	[System.Windows.Automation.Automation]::RemoveAutomationEventHandler(
		[System.Windows.Automation.TextPattern]::TextSelectionChangedEvent,
		$textField,
		[PoemUiaEventCounter]::TextHandler
	)
    if ([PoemUiaEventCounter]::TextCount -lt 2) { throw 'UIA text change notifications were not raised' }
	$password = Find-POEMElement 'gallery.password'
	if (-not $password.Current.IsPassword) { throw 'password control did not advertise IsPassword' }
	$patternObject = $null
	if ($password.TryGetCurrentPattern([System.Windows.Automation.ValuePattern]::Pattern, [ref]$patternObject)) {
		throw 'password control exposed ValuePattern'
	}
	$patternObject = $null
	if ($password.TryGetCurrentPattern([System.Windows.Automation.TextPattern]::Pattern, [ref]$patternObject)) {
		throw 'password control exposed TextPattern'
	}
	$automation = Invoke-RestMethod -Uri 'http://127.0.0.1:47831/components'
	$passwordNode = $automation.flat | Where-Object id -eq 'gallery.password'
	if ($passwordNode.text -or $passwordNode.value -or $passwordNode.selection_start -ne 0 -or $passwordNode.selection_end -ne 0) {
		throw 'password plaintext or length leaked through POEM automation'
	}

	$tabs = Find-POEMElement 'gallery.tabs'
	$tabSelection = [System.Windows.Automation.SelectionPattern]$tabs.GetCurrentPattern(
		[System.Windows.Automation.SelectionPattern]::Pattern
	)
	if ($tabSelection.Current.CanSelectMultiple -or -not $tabSelection.Current.IsSelectionRequired) {
		throw 'tab-list selection contract is incorrect'
	}
	if ($tabSelection.Current.GetSelection()[0].Current.AutomationId -ne 'gallery.tabs/inputs') {
		throw 'tab-list selected item is incorrect'
	}
	$inputsTab.SetFocus()
	Start-Sleep -Milliseconds 350
	$inputsTab = Find-POEMElement 'gallery.tabs/inputs'
	if (-not $inputsTab.Current.HasKeyboardFocus) {
		throw 'tab focus did not reach the portable semantic tree'
	}
	Invoke-RestMethod -Method Post -Uri 'http://127.0.0.1:47831/press-key' -ContentType 'application/json' -Body '{"key":"Right"}' | Out-Null
	Start-Sleep -Milliseconds 350
	$tabs = Find-POEMElement 'gallery.tabs'
	$tabSelection = [System.Windows.Automation.SelectionPattern]$tabs.GetCurrentPattern(
		[System.Windows.Automation.SelectionPattern]::Pattern
	)
	$keyboardSelectedTab = $tabSelection.Current.GetSelection()[0].Current.AutomationId
	if ($keyboardSelectedTab -ne 'gallery.tabs/navigation') {
		throw "tab keyboard navigation selected '$keyboardSelectedTab'"
	}
	$navigationTab = Find-POEMElement 'gallery.tabs/navigation'
	([System.Windows.Automation.SelectionItemPattern]$navigationTab.GetCurrentPattern(
		[System.Windows.Automation.SelectionItemPattern]::Pattern
	)).Select()
	Start-Sleep -Milliseconds 600
	$tree = Find-POEMElement 'gallery.tree'
	$frameworkItem = Find-POEMElement 'gallery.tree/framework'
	$nestedComponents = $frameworkItem.FindFirst(
		[System.Windows.Automation.TreeScope]::Children,
		[System.Windows.Automation.PropertyCondition]::new(
			[System.Windows.Automation.AutomationElement]::AutomationIdProperty,
			'gallery.tree/components'
		)
	)
	if ($null -eq $nestedComponents) {
		throw 'tree semantics flattened an expanded child outside its parent'
	}
	$frameworkItem.SetFocus()
	Start-Sleep -Milliseconds 200
	$frameworkItem = Find-POEMElement 'gallery.tree/framework'
	if (-not $frameworkItem.Current.HasKeyboardFocus -or $tree.Current.HasKeyboardFocus) {
		throw 'tree active-item focus remained on the owner or missed the item'
	}
	Invoke-RestMethod -Method Post -Uri 'http://127.0.0.1:47831/press-key' -ContentType 'application/json' -Body '{"key":"Right"}' | Out-Null
	Invoke-RestMethod -Method Post -Uri 'http://127.0.0.1:47831/press-key' -ContentType 'application/json' -Body '{"key":"Down"}' | Out-Null
	Start-Sleep -Milliseconds 250
	$themesItem = Find-POEMElement 'gallery.tree/themes'
	$tree = Find-POEMElement 'gallery.tree'
	$treeSelection = [System.Windows.Automation.SelectionPattern]$tree.GetCurrentPattern(
		[System.Windows.Automation.SelectionPattern]::Pattern
	)
	if (-not $themesItem.Current.HasKeyboardFocus -or $treeSelection.Current.GetSelection()[0].Current.AutomationId -ne 'gallery.tree/themes') {
		throw 'tree Right/Down navigation did not reach Themes'
	}
	Invoke-RestMethod -Method Post -Uri 'http://127.0.0.1:47831/press-key' -ContentType 'application/json' -Body '{"key":"Left"}' | Out-Null
	Start-Sleep -Milliseconds 200
	$frameworkItem = Find-POEMElement 'gallery.tree/framework'
	if (-not $frameworkItem.Current.HasKeyboardFocus) {
		throw 'tree Left navigation did not return to the parent'
	}
	$treeKeyboardItem = $frameworkItem.Current.AutomationId
	$accessibilityHeader = Find-POEMElement 'gallery.accordion/accessibility'
	$accessibilityExpand = [System.Windows.Automation.ExpandCollapsePattern]$accessibilityHeader.GetCurrentPattern(
		[System.Windows.Automation.ExpandCollapsePattern]::Pattern
	)
	if ($accessibilityExpand.Current.ExpandCollapseState -ne [System.Windows.Automation.ExpandCollapseState]::Collapsed) {
		throw 'accordion accessibility header was not initially collapsed'
	}
	$accessibilityHeader.SetFocus()
	Start-Sleep -Milliseconds 250
	$accessibilityHeader = Find-POEMElement 'gallery.accordion/accessibility'
	if (-not $accessibilityHeader.Current.HasKeyboardFocus) {
		throw 'accordion header focus did not reach the semantic descendant'
	}
	Invoke-RestMethod -Method Post -Uri 'http://127.0.0.1:47831/press-key' -ContentType 'application/json' -Body '{"key":"Home"}' | Out-Null
	Start-Sleep -Milliseconds 200
	$appearanceHeader = Find-POEMElement 'gallery.accordion/appearance'
	if (-not $appearanceHeader.Current.HasKeyboardFocus) {
		throw 'accordion Home did not focus the first enabled header'
	}
	Invoke-RestMethod -Method Post -Uri 'http://127.0.0.1:47831/press-key' -ContentType 'application/json' -Body '{"key":"End"}' | Out-Null
	Invoke-RestMethod -Method Post -Uri 'http://127.0.0.1:47831/press-key' -ContentType 'application/json' -Body '{"key":"Enter"}' | Out-Null
	Start-Sleep -Milliseconds 300
	$accessibilityHeader = Find-POEMElement 'gallery.accordion/accessibility'
	$accessibilityExpand = [System.Windows.Automation.ExpandCollapsePattern]$accessibilityHeader.GetCurrentPattern(
		[System.Windows.Automation.ExpandCollapsePattern]::Pattern
	)
	if (-not $accessibilityHeader.Current.HasKeyboardFocus -or $accessibilityExpand.Current.ExpandCollapseState -ne [System.Windows.Automation.ExpandCollapseState]::Expanded) {
		throw 'accordion End/Enter did not focus and expand the final header'
	}
	$accordionKeyboardHeader = $accessibilityHeader.Current.AutomationId
	$pagination = Find-POEMElement 'gallery.pagination'
	$pageThree = Find-POEMElement 'gallery.pagination/page/3'
	$pageThree.SetFocus()
	Start-Sleep -Milliseconds 200
	$pageThree = Find-POEMElement 'gallery.pagination/page/3'
	$pagination = Find-POEMElement 'gallery.pagination'
	if (-not $pageThree.Current.HasKeyboardFocus -or $pagination.Current.HasKeyboardFocus) {
		throw 'pagination focus did not land exclusively on page 3'
	}
	Invoke-RestMethod -Method Post -Uri 'http://127.0.0.1:47831/press-key' -ContentType 'application/json' -Body '{"key":"Right"}' | Out-Null
	Start-Sleep -Milliseconds 200
	$pageFour = Find-POEMElement 'gallery.pagination/page/4'
	if (-not $pageFour.Current.HasKeyboardFocus) {
		throw 'pagination Right did not advance controlled active focus to page 4'
	}
	Invoke-RestMethod -Method Post -Uri 'http://127.0.0.1:47831/press-key' -ContentType 'application/json' -Body '{"key":"End"}' | Out-Null
	Start-Sleep -Milliseconds 200
	$pageTwelve = Find-POEMElement 'gallery.pagination/page/12'
	if (-not $pageTwelve.Current.HasKeyboardFocus) {
		throw 'pagination End did not focus the final page'
	}
	Invoke-RestMethod -Method Post -Uri 'http://127.0.0.1:47831/press-key' -ContentType 'application/json' -Body '{"key":"Home"}' | Out-Null
	Start-Sleep -Milliseconds 200
	$pageOne = Find-POEMElement 'gallery.pagination/page/1'
	if (-not $pageOne.Current.HasKeyboardFocus) {
		throw 'pagination Home did not focus the first page'
	}
	$paginationKeyboardPage = $pageOne.Current.AutomationId

	$dataGrid = Find-POEMElement 'gallery.table'
	$gridPattern = [System.Windows.Automation.GridPattern]$dataGrid.GetCurrentPattern(
		[System.Windows.Automation.GridPattern]::Pattern
	)
	if ($gridPattern.Current.RowCount -ne 4 -or $gridPattern.Current.ColumnCount -ne 3) {
		throw "data-grid dimensions are $($gridPattern.Current.RowCount)x$($gridPattern.Current.ColumnCount)"
	}
	$gridShape = "$($gridPattern.Current.RowCount)x$($gridPattern.Current.ColumnCount)"
	$cell = $gridPattern.GetItem(1, 2)
	$gridItem = [System.Windows.Automation.GridItemPattern]$cell.GetCurrentPattern(
		[System.Windows.Automation.GridItemPattern]::Pattern
	)
	if ($gridItem.Current.Row -ne 1 -or $gridItem.Current.Column -ne 2 -or
		$gridItem.Current.ContainingGrid.Current.AutomationId -ne 'gallery.table') {
		throw 'GridItem coordinates or containing grid are incorrect'
	}
	$cellValuePattern = [System.Windows.Automation.ValuePattern]$cell.GetCurrentPattern(
		[System.Windows.Automation.ValuePattern]::Pattern
	)
	$cellValue = $cellValuePattern.Current.Value
	if ($cellValue -ne 'New' -or -not $cellValuePattern.Current.IsReadOnly) {
		throw "grid cell value is '$cellValue' or was not read-only"
	}
	$tablePattern = [System.Windows.Automation.TablePattern]$dataGrid.GetCurrentPattern(
		[System.Windows.Automation.TablePattern]::Pattern
	)
	if ($tablePattern.Current.GetColumnHeaders().Count -ne 3) { throw 'TablePattern did not return three column headers' }
	$tableSelection = [System.Windows.Automation.SelectionPattern]$dataGrid.GetCurrentPattern(
		[System.Windows.Automation.SelectionPattern]::Pattern
	)
	if ($tableSelection.Current.GetSelection()[0].Current.AutomationId -ne 'gallery.table/row/alpha') {
		throw 'data-grid initial selected row is incorrect'
	}
	$betaRow = Find-POEMElement 'gallery.table/row/beta'
	([System.Windows.Automation.SelectionItemPattern]$betaRow.GetCurrentPattern(
		[System.Windows.Automation.SelectionItemPattern]::Pattern
	)).Select()
	Start-Sleep -Milliseconds 500
	$dataGrid = Find-POEMElement 'gallery.table'
	$tableSelection = [System.Windows.Automation.SelectionPattern]$dataGrid.GetCurrentPattern(
		[System.Windows.Automation.SelectionPattern]::Pattern
	)
	if ($tableSelection.Current.GetSelection()[0].Current.AutomationId -ne 'gallery.table/row/beta') {
		throw 'data-grid row selection did not reach Go'
	}
	$betaRow = Find-POEMElement 'gallery.table/row/beta'
	$betaRow.SetFocus()
	Start-Sleep -Milliseconds 350
	$betaRow = Find-POEMElement 'gallery.table/row/beta'
	if (-not $betaRow.Current.HasKeyboardFocus) {
		throw 'data-grid row focus did not reach the portable semantic tree'
	}
	Invoke-RestMethod -Method Post -Uri 'http://127.0.0.1:47831/press-key' -ContentType 'application/json' -Body '{"key":"Down"}' | Out-Null
	Start-Sleep -Milliseconds 350
	$dataGrid = Find-POEMElement 'gallery.table'
	$tableSelection = [System.Windows.Automation.SelectionPattern]$dataGrid.GetCurrentPattern(
		[System.Windows.Automation.SelectionPattern]::Pattern
	)
	$keyboardSelectedRow = $tableSelection.Current.GetSelection()[0].Current.AutomationId
	if ($keyboardSelectedRow -ne 'gallery.table/row/gamma') {
		throw "data-grid keyboard navigation selected '$keyboardSelectedRow'"
	}
	$viewport = Find-POEMElement 'gallery.scroll'
	$scrollPattern = [System.Windows.Automation.ScrollPattern]$viewport.GetCurrentPattern(
		[System.Windows.Automation.ScrollPattern]::Pattern
	)
	if (-not $scrollPattern.Current.VerticallyScrollable -or $scrollPattern.Current.VerticalViewSize -ge 100) {
		throw 'scroll viewport did not expose a vertical scroll range'
	}
	$propertyEventsBeforeScroll = [PoemUiaEventCounter]::Count
	[System.Windows.Automation.Automation]::AddAutomationPropertyChangedEventHandler(
		$viewport,
		[System.Windows.Automation.TreeScope]::Element,
		[PoemUiaEventCounter]::Handler,
		[System.Windows.Automation.ScrollPattern]::VerticalScrollPercentProperty
	)
	$scrollPattern.SetScrollPercent(
		[System.Windows.Automation.ScrollPattern]::NoScroll,
		100
	)
	Start-Sleep -Milliseconds 500
	[System.Windows.Automation.Automation]::RemoveAutomationPropertyChangedEventHandler(
		$viewport,
		[PoemUiaEventCounter]::Handler
	)
	if ([PoemUiaEventCounter]::Count -le $propertyEventsBeforeScroll) {
		throw 'scroll property-change notification was not raised'
	}
	$viewport = Find-POEMElement 'gallery.scroll'
	$scrollPattern = [System.Windows.Automation.ScrollPattern]$viewport.GetCurrentPattern(
		[System.Windows.Automation.ScrollPattern]::Pattern
	)
	if ([math]::Abs($scrollPattern.Current.VerticalScrollPercent - 100) -gt 0.1) {
		throw "scroll action stopped at $($scrollPattern.Current.VerticalScrollPercent)%"
	}
	$scrollVerifiedPercent = $scrollPattern.Current.VerticalScrollPercent
	$nestedTable = $viewport.FindFirst(
		[System.Windows.Automation.TreeScope]::Descendants,
		[System.Windows.Automation.PropertyCondition]::new(
			[System.Windows.Automation.AutomationElement]::AutomationIdProperty,
			'gallery.table'
		)
	)
	if ($null -eq $nestedTable) { throw 'semantic hierarchy did not nest the table under its viewport' }
	$viewportRect = $viewport.Current.BoundingRectangle
	$tableRect = $nestedTable.Current.BoundingRectangle
	if ($nestedTable.Current.IsOffscreen -or $tableRect.Top -lt $viewportRect.Top -or $tableRect.Bottom -gt $viewportRect.Bottom) {
		throw 'scrolled table semantic bounds were not transformed and clipped to the viewport'
	}
	$scrolledTabs = Find-POEMElement 'gallery.tabs'
	if (-not $scrolledTabs.Current.IsOffscreen) { throw 'scrolled-out tabs were not marked offscreen' }

	$feedbackTab = Find-POEMElement 'gallery.tabs/feedback'
	([System.Windows.Automation.SelectionItemPattern]$feedbackTab.GetCurrentPattern(
		[System.Windows.Automation.SelectionItemPattern]::Pattern
	)).Select()
	Start-Sleep -Milliseconds 400
	Invoke-RestMethod -Method Post -Uri 'http://127.0.0.1:47831/click' -ContentType 'application/json' -Body '{"id":"gallery.menu-button"}' | Out-Null
	Start-Sleep -Milliseconds 400
	$newMenuItem = Find-POEMElement 'gallery.menu/new'
	if (-not $newMenuItem.Current.HasKeyboardFocus) {
		throw 'focused menu overlay did not focus its first enabled item'
	}
	Invoke-RestMethod -Method Post -Uri 'http://127.0.0.1:47831/press-key' -ContentType 'application/json' -Body '{"key":"Down"}' | Out-Null
	Start-Sleep -Milliseconds 300
	$openMenuItem = Find-POEMElement 'gallery.menu/open'
	if (-not $openMenuItem.Current.HasKeyboardFocus) {
		throw 'menu Down navigation did not focus Open'
	}
	$menuKeyboardItem = $openMenuItem.Current.AutomationId
	Invoke-RestMethod -Method Post -Uri 'http://127.0.0.1:47831/press-key' -ContentType 'application/json' -Body '{"key":"End"}' | Out-Null
	Invoke-RestMethod -Method Post -Uri 'http://127.0.0.1:47831/press-key' -ContentType 'application/json' -Body '{"key":"Enter"}' | Out-Null
	Start-Sleep -Milliseconds 400
	$menuCondition = [System.Windows.Automation.PropertyCondition]::new(
		[System.Windows.Automation.AutomationElement]::AutomationIdProperty,
		'gallery.menu'
	)
	if ($null -ne $root.FindFirst([System.Windows.Automation.TreeScope]::Descendants, $menuCondition)) {
		throw 'activated menu did not dismiss'
	}
	$menuButton = Find-POEMElement 'gallery.menu-button'
	if (-not $menuButton.Current.HasKeyboardFocus) {
		throw 'menu dismissal did not restore launcher focus'
	}

    [PSCustomObject]@{
        RootName = $root.Current.Name
        SliderValue = $sliderVerifiedValue
        ToggleState = $toggleVerifiedState
		ShortcutValue = $shortcutVerifiedValue
		MnemonicAccessKey = $mnemonicAccessKey
		SemanticSelectorTarget = $semanticSelectorTarget
		RelationshipTargets = $relationshipTargets
        PropertyEvents = [PoemUiaEventCounter]::Count
        TextSelection = $selectedText
        RuneSelection = "$($automationNode.selection_start):$($automationNode.selection_end)"
		TextEvents = [PoemUiaEventCounter]::TextCount
		TabKeyboardSelection = $keyboardSelectedTab
		ComboKeyboardValue = $comboKeyboardValue
		CalendarGridShape = $calendarGridShape
		CalendarKeyboardValue = $calendarKeyboardValue
		AnchoredOverlayDismissal = $anchoredOverlayDismissal
		OutsideClickTarget = $outsideClickTarget
		TreeKeyboardItem = $treeKeyboardItem
		AccordionKeyboardHeader = $accordionKeyboardHeader
		PaginationKeyboardPage = $paginationKeyboardPage
		GridShape = $gridShape
		GridCell = $cellValue
		GridKeyboardSelection = $keyboardSelectedRow
		MenuKeyboardItem = $menuKeyboardItem
		ScrollPercent = $scrollVerifiedPercent
        Result = 'passed'
    }
} finally {
    if (-not $process.HasExited) { Stop-Process -Id $process.Id -Force }
}
