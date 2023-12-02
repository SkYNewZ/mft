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

// deleteShortcutCmd represents the deleteShortcut command
var deleteShortcutCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a shortcut",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc := cmd.Context().Value(driveServiceContextKey{}).(*drive.Service)

		deleteByName, _ := cmd.Flags().GetBool("byName")
		deleteByID, _ := cmd.Flags().GetBool("byID")
		parentFolderID, _ := cmd.Flags().GetString("parentFolderID")

		switch {
		case deleteByName:
			f, err := findShortcutByName(cmd.Context(), svc, args[0], parentFolderID)
			if err != nil {
				return fmt.Errorf("unable to find shortcut: %w", err)
			}

			if err := deleteShortcutByID(cmd.Context(), svc, f.Id); err != nil {
				return fmt.Errorf("unable to delete shortcut: %w", err)
			}
		case deleteByID:
			if err := deleteShortcutByID(cmd.Context(), svc, args[0]); err != nil {
				return fmt.Errorf("unable to delete shortcut: %w", err)
			}
		}

		return nil
	},
}

func init() {
	driveCmd.AddCommand(deleteShortcutCmd)

	deleteShortcutCmd.Flags().Bool("byName", false, "Delete a shortcut by name")
	deleteShortcutCmd.Flags().Bool("byID", false, "Delete a shortcut by ID")
	deleteShortcutCmd.Flags().String("parentFolderID", "", "Parent folder ID to search for shortcuts")

	_ = deleteShortcutCmd.MarkFlagRequired("parentFolderID")
	deleteShortcutCmd.MarkFlagsMutuallyExclusive("byName", "byID")
	deleteShortcutCmd.MarkFlagsOneRequired("byName", "byID")
}

// deleteShortcutByID deletes a shortcut by ID
func deleteShortcutByID(ctx context.Context, srv *drive.Service, shortcutID string) error {
	return srv.Files.Delete(shortcutID).SupportsAllDrives(true).Context(ctx).Do()
}
