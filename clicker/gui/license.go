//go:build windows

package main

import (
	"errors"
	"strings"

	"experimental-clicker/license"
	"github.com/lxn/walk"
)

func ensureActivated() bool {
	if license.Activated() {
		return true
	}
	return showLicenseDialog()
}

func copyToClipboard(owner walk.Form, text string) {
	if err := walk.Clipboard().SetText(text); err != nil {
		walk.MsgBox(owner, "Copy failed", err.Error(), walk.MsgBoxIconWarning)
		return
	}
}

func showLicenseDialog() bool {
	machineID, err := license.MachineID()
	if err != nil {
		walk.MsgBox(nil, "BELARUS CHAMP CLICKER", err.Error(), walk.MsgBoxIconError)
		return false
	}
	displayID := license.FormatMachineID(machineID)

	dlg, err := walk.NewDialog(nil)
	if err != nil {
		walk.MsgBox(nil, "BELARUS CHAMP CLICKER", err.Error(), walk.MsgBoxIconError)
		return false
	}
	defer dlg.Dispose()

	if err := dlg.SetTitle("Activation / Активация"); err != nil {
		return false
	}
	if err := dlg.SetSize(walk.Size{Width: 560, Height: 420}); err != nil {
		return false
	}

	layout := walk.NewVBoxLayout()
	layout.SetMargins(walk.Margins{HNear: 16, VNear: 16, HFar: 16, VFar: 16})
	layout.SetSpacing(10)
	if err := dlg.SetLayout(layout); err != nil {
		return false
	}

	step1GB, err := walk.NewGroupBox(dlg)
	if err != nil {
		return false
	}
	if err := step1GB.SetTitle("Step 1 — Send this to the seller / Шаг 1 — отправьте продавцу"); err != nil {
		return false
	}
	step1Layout := walk.NewVBoxLayout()
	step1Layout.SetSpacing(8)
	if err := step1GB.SetLayout(step1Layout); err != nil {
		return false
	}

	machineRow, err := walk.NewComposite(step1GB)
	if err != nil {
		return false
	}
	machineHBox := walk.NewHBoxLayout()
	machineHBox.SetSpacing(8)
	if err := machineRow.SetLayout(machineHBox); err != nil {
		return false
	}

	machineValue, err := walk.NewLineEdit(machineRow)
	if err != nil {
		return false
	}
	if err := machineValue.SetReadOnly(true); err != nil {
		return false
	}
	if err := machineValue.SetText(displayID); err != nil {
		return false
	}
	if err := machineValue.SetMinMaxSize(walk.Size{Width: 140, Height: 0}, walk.Size{Width: 140, Height: 0}); err != nil {
		return false
	}

	copyIDBtn, err := walk.NewPushButton(machineRow)
	if err != nil {
		return false
	}
	if err := copyIDBtn.SetText("Copy ID / Копировать"); err != nil {
		return false
	}
	copyIDBtn.Clicked().Attach(func() {
		copyToClipboard(dlg, displayID)
		walk.MsgBox(dlg, "Copied / Скопировано", "Computer ID copied to clipboard.", walk.MsgBoxIconInformation)
	})

	step2GB, err := walk.NewGroupBox(dlg)
	if err != nil {
		return false
	}
	if err := step2GB.SetTitle("Step 2 — Enter code from seller / Шаг 2 — введите код"); err != nil {
		return false
	}
	step2Layout := walk.NewVBoxLayout()
	step2Layout.SetSpacing(8)
	if err := step2GB.SetLayout(step2Layout); err != nil {
		return false
	}

	codeEdit, err := walk.NewTextEdit(step2GB)
	if err != nil {
		return false
	}
	if err := codeEdit.SetMinMaxSize(walk.Size{Width: 0, Height: 80}, walk.Size{}); err != nil {
		return false
	}

	pasteBtn, err := walk.NewPushButton(step2GB)
	if err != nil {
		return false
	}
	if err := pasteBtn.SetText("Paste code / Вставить"); err != nil {
		return false
	}
	pasteBtn.Clicked().Attach(func() {
		text, err := walk.Clipboard().Text()
		if err != nil || text == "" {
			walk.MsgBox(dlg, "Paste", "Clipboard is empty.", walk.MsgBoxIconWarning)
			return
		}
		codeEdit.SetText(strings.TrimSpace(text))
	})

	btnRow, err := walk.NewComposite(dlg)
	if err != nil {
		return false
	}
	btnLayout := walk.NewHBoxLayout()
	btnLayout.SetSpacing(10)
	if err := btnRow.SetLayout(btnLayout); err != nil {
		return false
	}

	activateBtn, err := walk.NewPushButton(btnRow)
	if err != nil {
		return false
	}
	if err := activateBtn.SetText("Activate / Активировать"); err != nil {
		return false
	}

	exitBtn, err := walk.NewPushButton(btnRow)
	if err != nil {
		return false
	}
	if err := exitBtn.SetText("Exit / Выход"); err != nil {
		return false
	}

	var accepted bool

	activate := func() {
		if err := license.SaveCode(codeEdit.Text()); err != nil {
			msg := err.Error()
			switch {
			case errors.Is(err, license.ErrInvalidCode):
				msg = "Invalid activation code.\nPaste the full code starting with BCC-\n\nНеверный код. Вставьте полный код, начинается с BCC-"
			case errors.Is(err, license.ErrWrongMachine):
				msg = "This code is for a different computer.\nКод предназначен для другого компьютера."
			}
			walk.MsgBox(dlg, "Activation failed / Ошибка", msg, walk.MsgBoxIconWarning)
			return
		}
		accepted = true
		dlg.Accept()
	}

	activateBtn.Clicked().Attach(activate)
	exitBtn.Clicked().Attach(func() {
		dlg.Cancel()
	})

	if err := dlg.SetDefaultButton(activateBtn); err != nil {
		return false
	}
	if err := dlg.SetCancelButton(exitBtn); err != nil {
		return false
	}

	dlg.Run()
	if !accepted {
		return false
	}

	walk.MsgBox(nil, "BELARUS CHAMP CLICKER", "Activation successful. / Активация успешна.", walk.MsgBoxIconInformation)
	return true
}

func showLicenseInfo(parent walk.Form) {
	if !license.Activated() {
		showLicenseDialog()
		return
	}

	machineID, err := license.MachineID()
	if err != nil {
		walk.MsgBox(parent, "License", err.Error(), walk.MsgBoxIconError)
		return
	}
	displayID := license.FormatMachineID(machineID)
	walk.MsgBox(parent, "License / Лицензия",
		"Status: Activated / Статус: активировано\nComputer ID: "+displayID,
		walk.MsgBoxIconInformation)
}
