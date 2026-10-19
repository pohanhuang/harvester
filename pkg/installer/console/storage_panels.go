package console

import (
	"github.com/jroimartin/gocui"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/harvester/harvester/pkg/installer/config"
	"github.com/harvester/harvester/pkg/installer/widgets"
)

const (
	deferLonghornNo  = "no"
	deferLonghornYes = "yes"

	deferLonghornNote = "Longhorn is not installed and no data disk or HARV_LH_DEFAULT partition is created.\n" +
		"No default CSI storage is available after installation. Storage-dependent features\n" +
		"(monitoring persistence, k3k, VM state, upgrade) need the CSI drivers you name next.\n" +
		"Install and configure them after installation. Minimum root disk: 250 GiB."
	systemProvisionerNote  = "CSI provisioner for harvester-system-rwo (RWO volumes for system workloads), e.g. rook-ceph.rbd.csi.ceph.com"
	upgradeProvisionerNote = "CSI provisioner for harvester-upgrade-rwx (RWX volume for the upgrade repository), e.g. rook-ceph.cephfs.csi.ceph.com"
)

// showStoragePage asks about deferring Longhorn when creating a cluster. A
// joining node follows the cluster's InstallProfile instead.
func showStoragePage(c *Console) error {
	if c.config.Install.Mode != config.ModeCreate {
		return showDiskPage(c)
	}
	if err := createVerticalLocatorWithName(c)(askDeferLonghornPanel, 3); err != nil {
		return err
	}
	return showNext(c, askDeferLonghornPanel)
}

func addAskDeferLonghornPanel(c *Console) error {
	optionsFunc := func() ([]widgets.Option, error) {
		return []widgets.Option{
			{Value: deferLonghornNo, Text: "Install Longhorn (default)"},
			{Value: deferLonghornYes, Text: "Defer Longhorn installation"},
		}, nil
	}
	askV, err := widgets.NewSelect(c.Gui, askDeferLonghornPanel, "", optionsFunc)
	if err != nil {
		return err
	}
	closeThisPage := func() error {
		c.CloseElement(notePanel)
		return askV.Close()
	}
	askV.PreShow = func() error {
		askV.Value = deferLonghornNo
		if c.config.Install.DeferLonghornInstall {
			askV.Value = deferLonghornYes
		}
		if err := c.setContentByName(notePanel, deferLonghornNote); err != nil {
			return err
		}
		return c.setContentByName(titlePanel, "Choose default storage")
	}
	askV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyEnter: func(_ *gocui.Gui, _ *gocui.View) error {
			selected, err := askV.GetData()
			if err != nil {
				return err
			}
			c.config.Install.DeferLonghornInstall = selected == deferLonghornYes
			if err := closeThisPage(); err != nil {
				return err
			}
			if !c.config.Install.DeferLonghornInstall {
				c.config.Harvester.StorageClass.System.Provisioner = ""
				c.config.Harvester.StorageClass.Upgrade.Provisioner = ""
				return showDiskPage(c)
			}
			return showNext(c, systemProvisionerPanel)
		},
		gocui.KeyEsc: func(_ *gocui.Gui, _ *gocui.View) error {
			if err := closeThisPage(); err != nil {
				return err
			}
			return showPasswordPage(c)
		},
	}
	c.AddElement(askDeferLonghornPanel, askV)
	return nil
}

// addProvisionerPanel adds an input for a CSI provisioner name.
func addProvisionerPanel(c *Console, name, label, note string, field func() *string, prev, next func() error, invalidMsg string) error {
	inputV, err := widgets.NewInput(c.Gui, name, label, false)
	if err != nil {
		return err
	}
	inputV.PreShow = func() error {
		c.Gui.Cursor = true
		inputV.Value = *field()
		if err := c.setContentByName(notePanel, note); err != nil {
			return err
		}
		return c.setContentByName(titlePanel, "Configure CSI provisioner")
	}
	closeThisPage := func() error {
		c.CloseElement(notePanel)
		c.CloseElement(validatorPanel)
		return inputV.Close()
	}
	inputV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyEnter: func(_ *gocui.Gui, _ *gocui.View) error {
			v, err := inputV.GetData()
			if err != nil {
				return err
			}
			if errs := validation.IsDNS1123Subdomain(v); v == "" || len(errs) > 0 {
				return c.setContentByName(validatorPanel, invalidMsg)
			}
			*field() = v
			if err := closeThisPage(); err != nil {
				return err
			}
			return next()
		},
		gocui.KeyEsc: func(_ *gocui.Gui, _ *gocui.View) error {
			if err := closeThisPage(); err != nil {
				return err
			}
			return prev()
		},
	}
	c.AddElement(name, inputV)
	return nil
}

func addProvisionerPanels(c *Console) error {
	if err := addProvisionerPanel(c, systemProvisionerPanel, "System provisioner", systemProvisionerNote,
		func() *string { return &c.config.Harvester.StorageClass.System.Provisioner },
		func() error { return showNext(c, askDeferLonghornPanel) },
		func() error { return showNext(c, upgradeProvisionerPanel) },
		ErrMsgSystemProvisionerInvalid); err != nil {
		return err
	}
	return addProvisionerPanel(c, upgradeProvisionerPanel, "Upgrade provisioner", upgradeProvisionerNote,
		func() *string { return &c.config.Harvester.StorageClass.Upgrade.Provisioner },
		func() error { return showNext(c, systemProvisionerPanel) },
		func() error { return showDiskPage(c) },
		ErrMsgUpgradeProvisionerInvalid)
}
