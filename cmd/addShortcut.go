/*
Copyright © 2023 Quentin Lemaire

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"google.golang.org/api/drive/v3"
)

// addShortcutCmd represents the addShortcut command
var addShortcutCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a shortcut to a folder",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc := cmd.Context().Value(driveServiceContextKey{}).(*drive.Service)

		parentFolderID, _ := cmd.Flags().GetString("parentFolderID")
		shortcutName, _ := cmd.Flags().GetString("name")

		addByName, _ := cmd.Flags().GetBool("byName")
		addByID, _ := cmd.Flags().GetBool("byID")
		switch {
		case addByName:
			targetFolder, err := findByName(cmd.Context(), svc, args[0], parentFolderID, MimeTypeFolder)
			if err != nil {
				return err
			}

			f, err := createShortcutFolder(cmd.Context(), svc, shortcutName, parentFolderID, targetFolder.Id)
			if err != nil {
				return fmt.Errorf("unable to create shortcut folder: %w", err)
			}

			cmd.Println(f.Id)
		case addByID:
			f, err := createShortcutFolder(cmd.Context(), svc, shortcutName, parentFolderID, args[0])
			if err != nil {
				return fmt.Errorf("unable to create shortcut folder: %w", err)
			}

			cmd.Println(f.Id)
		}

		return nil
	},
}

func init() {
	driveCmd.AddCommand(addShortcutCmd)

	addShortcutCmd.Flags().String("parentFolderID", "", "ID of the parent folder")
	addShortcutCmd.Flags().String("name", "# Latest", "Name of the shortcut")

	addShortcutCmd.Flags().Bool("byName", false, "Find target folder by name")
	addShortcutCmd.Flags().Bool("byID", true, "Find target folder by ID")
	addShortcutCmd.MarkFlagsMutuallyExclusive("byName", "byID")
	addShortcutCmd.MarkFlagsOneRequired("byName", "byID")

	_ = addShortcutCmd.MarkFlagRequired("parentFolderID")
	_ = addShortcutCmd.MarkFlagRequired("targetID")
}

// createShortcutFolder creates a shortcut folder on Google Drive
func createShortcutFolder(ctx context.Context, srv *drive.Service, shortcutName string, parentID string, targetID string) (*drive.File, error) {
	f := &drive.File{
		Name:            shortcutName,
		MimeType:        MimeTypeShortcut,
		Parents:         []string{parentID},
		ShortcutDetails: &drive.FileShortcutDetails{TargetId: targetID},
	}

	// Create the shortcut folder
	file, err := srv.Files.Create(f).SupportsAllDrives(true).Context(ctx).Do()
	if err != nil {
		return nil, err
	}

	return file, nil
}
