//go:build windows

//go:generate go run github.com/akavel/rsrc@v0.10.2 -manifest ../../gui/app.manifest -o rsrc.syso

package main

import (
	"fmt"

	"experimental-clicker/license"
	"github.com/lxn/walk"
)

func main() {
	if err := run(); err != nil {
		walk.MsgBox(nil, "License Manager", err.Error(), walk.MsgBoxIconError)
	}
}

func run() error {
	mw, err := walk.NewMainWindow()
	if err != nil {
		return err
	}

	if err := mw.SetTitle("Belarus Champ Clicker - License Manager"); err != nil {
		return err
	}
	if err := mw.SetSize(walk.Size{Width: 580, Height: 560}); err != nil {
		return err
	}

	root := walk.NewVBoxLayout()
	root.SetMargins(walk.Margins{HNear: 12, VNear: 12, HFar: 12, VFar: 12})
	root.SetSpacing(10)
	if err := mw.SetLayout(root); err != nil {
		return err
	}

	header, err := walk.NewLabel(mw)
	if err != nil {
		return err
	}
	if err := header.SetText(
		"Author tool — follow steps 1, 2, 3 in order.\n" +
			"Do not ship this program to users.",
	); err != nil {
		return err
	}

	privPath, pubPath := license.KeyPaths()

	step1, err := walk.NewGroupBox(mw)
	if err != nil {
		return err
	}
	if err := step1.SetTitle("Step 1 — One-time setup (do this first)"); err != nil {
		return err
	}
	step1Layout := walk.NewVBoxLayout()
	step1Layout.SetSpacing(8)
	if err := step1.SetLayout(step1Layout); err != nil {
		return err
	}

	step1Hint, err := walk.NewLabel(step1)
	if err != nil {
		return err
	}
	if err := step1Hint.SetText(
		"Click Create signing keys.\n" +
			"The user app and release ZIP will be built automatically (may take a minute).",
	); err != nil {
		return err
	}

	statusLabel, err := walk.NewLabel(step1)
	if err != nil {
		return err
	}

	createKeysBtn, err := walk.NewPushButton(step1)
	if err != nil {
		return err
	}
	if err := createKeysBtn.SetText("Create signing keys"); err != nil {
		return err
	}

	step2, err := walk.NewGroupBox(mw)
	if err != nil {
		return err
	}
	if err := step2.SetTitle("Step 2 — Customer Computer ID"); err != nil {
		return err
	}
	step2Layout := walk.NewVBoxLayout()
	step2Layout.SetSpacing(8)
	if err := step2.SetLayout(step2Layout); err != nil {
		return err
	}

	step2Hint, err := walk.NewLabel(step2)
	if err != nil {
		return err
	}
	if err := step2Hint.SetText(
		"The customer runs the clicker, copies their Computer ID, and sends it to you.\n" +
			"For your own PC: click Use my ID.",
	); err != nil {
		return err
	}

	idRow, err := walk.NewComposite(step2)
	if err != nil {
		return err
	}
	idHBox := walk.NewHBoxLayout()
	idHBox.SetSpacing(8)
	if err := idRow.SetLayout(idHBox); err != nil {
		return err
	}

	machineEdit, err := walk.NewLineEdit(idRow)
	if err != nil {
		return err
	}

	useLocalBtn, err := walk.NewPushButton(idRow)
	if err != nil {
		return err
	}
	if err := useLocalBtn.SetText("Use my ID"); err != nil {
		return err
	}

	localID, err := license.MachineID()
	if err != nil {
		return fmt.Errorf("machine id: %w", err)
	}
	localDisplay := license.FormatMachineID(localID)
	useLocalBtn.Clicked().Attach(func() {
		machineEdit.SetText(localDisplay)
	})

	noteLabel, err := walk.NewLabel(step2)
	if err != nil {
		return err
	}
	if err := noteLabel.SetText("Optional note (customer name):"); err != nil {
		return err
	}

	noteEdit, err := walk.NewLineEdit(step2)
	if err != nil {
		return err
	}

	step3, err := walk.NewGroupBox(mw)
	if err != nil {
		return err
	}
	if err := step3.SetTitle("Step 3 — Generate code and send to customer"); err != nil {
		return err
	}
	step3Layout := walk.NewVBoxLayout()
	step3Layout.SetSpacing(8)
	if err := step3.SetLayout(step3Layout); err != nil {
		return err
	}

	step3Hint, err := walk.NewLabel(step3)
	if err != nil {
		return err
	}
	if err := step3Hint.SetText(
		"1. Click Generate code.\n" +
			"2. Click Copy code and send it to the customer.\n" +
			"3. Customer pastes the code in the clicker activation window.",
	); err != nil {
		return err
	}

	actionRow, err := walk.NewComposite(step3)
	if err != nil {
		return err
	}
	actionHBox := walk.NewHBoxLayout()
	actionHBox.SetSpacing(8)
	if err := actionRow.SetLayout(actionHBox); err != nil {
		return err
	}

	generateBtn, err := walk.NewPushButton(actionRow)
	if err != nil {
		return err
	}
	if err := generateBtn.SetText("Generate code"); err != nil {
		return err
	}

	copyCodeBtn, err := walk.NewPushButton(actionRow)
	if err != nil {
		return err
	}
	if err := copyCodeBtn.SetText("Copy code"); err != nil {
		return err
	}

	codeEdit, err := walk.NewTextEdit(step3)
	if err != nil {
		return err
	}
	if err := codeEdit.SetReadOnly(true); err != nil {
		return err
	}
	if err := codeEdit.SetMinMaxSize(walk.Size{Width: 0, Height: 80}, walk.Size{}); err != nil {
		return err
	}

	keysReady := func() bool {
		_, err := license.LoadPrivateKey(privPath)
		return err == nil
	}

	refreshStep1 := func() {
		if keysReady() {
			statusLabel.SetText("Status: signing keys ready. User release built. Continue to Step 2.")
			generateBtn.SetEnabled(true)
			return
		}
		statusLabel.SetText("Status: signing keys not created yet.")
		generateBtn.SetEnabled(false)
	}

	createKeysBtn.Clicked().Attach(func() {
		if keysReady() {
			walk.MsgBox(mw, "Step 1", "Signing keys already exist. Continue to Step 2.", walk.MsgBoxIconInformation)
			return
		}

		createKeysBtn.SetEnabled(false)
		statusLabel.SetText("Status: creating signing keys...")
		if err := license.GenerateKeyPair(privPath, pubPath); err != nil {
			createKeysBtn.SetEnabled(true)
			walk.MsgBox(mw, "Step 1 failed", err.Error(), walk.MsgBoxIconError)
			refreshStep1()
			return
		}

		statusLabel.SetText("Status: building clicker and release ZIP (please wait)...")
		mw.SetEnabled(false)

		go func() {
			zipPath, err := runReleaseBuild()
			mw.Synchronize(func() {
				mw.SetEnabled(true)
				createKeysBtn.SetEnabled(true)
				if err != nil {
					walk.MsgBox(mw, "Build failed", err.Error(), walk.MsgBoxIconError)
					refreshStep1()
					return
				}
				refreshStep1()
				walk.MsgBox(mw, "Step 1 done",
					"Signing keys created and user release rebuilt.\n\nReady to ship:\n"+zipPath,
					walk.MsgBoxIconInformation)
			})
		}()
	})

	generateBtn.Clicked().Attach(func() {
		if !keysReady() {
			walk.MsgBox(mw, "Step 1 required", "Create signing keys in Step 1 first.", walk.MsgBoxIconWarning)
			return
		}
		machineID := machineEdit.Text()
		if machineID == "" {
			walk.MsgBox(mw, "Step 2 required", "Enter the customer Computer ID first.", walk.MsgBoxIconWarning)
			return
		}
		priv, err := license.LoadPrivateKey(privPath)
		if err != nil {
			walk.MsgBox(mw, "Step 1 required", "Create signing keys in Step 1 first.", walk.MsgBoxIconWarning)
			return
		}
		code, err := license.IssueCode(priv, machineID, noteEdit.Text())
		if err != nil {
			walk.MsgBox(mw, "Generate failed", err.Error(), walk.MsgBoxIconError)
			return
		}
		codeEdit.SetText(code)
		if err := walk.Clipboard().SetText(code); err != nil {
			walk.MsgBox(mw, "Generated", "Code generated but copy failed.\nUse Copy code.", walk.MsgBoxIconWarning)
			return
		}
		walk.MsgBox(mw, "Code generated", "Code copied to clipboard.\nSend it to the customer.", walk.MsgBoxIconInformation)
	})

	copyCodeBtn.Clicked().Attach(func() {
		code := codeEdit.Text()
		if code == "" {
			walk.MsgBox(mw, "Step 3", "Generate a code first.", walk.MsgBoxIconWarning)
			return
		}
		if err := walk.Clipboard().SetText(code); err != nil {
			walk.MsgBox(mw, "Copy failed", err.Error(), walk.MsgBoxIconError)
			return
		}
		walk.MsgBox(mw, "Copied", "Code copied. Send it to the customer.", walk.MsgBoxIconInformation)
	})

	refreshStep1()

	mw.Show()
	mw.Run()
	return nil
}
