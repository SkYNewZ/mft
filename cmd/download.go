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
	"archive/zip"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/chromedp"

	"github.com/spf13/cobra"
)

// downloadCmd represents the download command
var downloadCmd = &cobra.Command{
	Use:   "download",
	Short: "Download all FFESSM MFT documents",
	RunE: func(cmd *cobra.Command, args []string) error {
		outputPath, _ := cmd.Flags().GetString("output")
		isDebug, _ := cmd.Flags().GetBool("debug")

		logger := configureLogger(isDebug)
		out, err := downloadFiles(cmd.Context(), logger.WithGroup("chromedp"), outputPath)
		if err != nil {
			return fmt.Errorf("failed to download files: %w", err)
		}

		if err := unzipArchive(logger.WithGroup("unzip"), out, outputPath); err != nil {
			return fmt.Errorf("failed to unzip archive: %w", err)
		}

		if err := removeDownloadedFiles(logger.WithGroup("cleanup"), out); err != nil {
			return fmt.Errorf("failed to remove downloaded files: %w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(downloadCmd)

	wd, _ := os.Getwd()
	downloadCmd.PersistentFlags().String("output", filepath.Join(wd, "downloads"), "output path for PDF files")
	downloadCmd.PersistentFlags().Bool("debug", false, "enable debug mode")
}

// downloadFiles will navigate to the FFESSM MFT website, select all documents and download them as zip
// returns the path to the downloaded zip file
func downloadFiles(ctx context.Context, logger *slog.Logger, outputPath string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	options := append(
		chromedp.DefaultExecAllocatorOptions[:],
		chromedp.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36"),
		//chromedp.Flag("headless", false),
	)

	// setup options
	ctx, cancel = chromedp.NewExecAllocator(ctx, options...)
	defer cancel()

	// setup chrome instance config
	ctx, cancel = chromedp.NewContext(ctx,
		chromedp.WithLogf(logger.Info),
		chromedp.WithDebugf(logger.Debug),
		chromedp.WithErrorf(logger.Error),
	)
	defer cancel()

	// set up a channel, so we can block later while we monitor the download progress
	done := make(chan string, 1)
	chromedp.ListenTarget(ctx, func(v interface{}) {
		switch ev := v.(type) {
		case *browser.EventDownloadProgress:
			logger := logger.With(slog.String("guid", ev.GUID), slog.String("state", ev.State.String()))

			completed := "(unknown)"
			if ev.TotalBytes != 0 {
				completed = fmt.Sprintf("%0.2f%%", ev.ReceivedBytes/ev.TotalBytes*100.0)
			}

			logger.Info("download in progress", slog.String("completed", completed))
			if ev.State == browser.DownloadProgressStateCompleted {
				done <- ev.GUID
				close(done)
			}

		case *browser.EventDownloadWillBegin:
			logger := logger.With(slog.String("guid", ev.GUID), slog.String("url", ev.URL), slog.String("suggestedFilename", ev.SuggestedFilename))
			logger.Info("download started")
		}
	})

	if err := chromedp.Run(ctx,
		browser.SetDownloadBehavior(browser.SetDownloadBehaviorBehaviorAllowAndName).
			WithDownloadPath(outputPath).
			WithEventsEnabled(true),
		chromedp.Navigate("https://mft.ffessm.fr/pages/documents"),
		chromedp.Sleep(time.Second*5), // let documents to be displayed
		chromedp.WaitVisible("k-grid0-select-all", chromedp.ByID),
		chromedp.Click("k-grid0-select-all", chromedp.ByID),
		chromedp.WaitVisible("//button[normalize-space() = 'Télécharger les éléments sélectionnés']", chromedp.BySearch),
		chromedp.Click("//button[normalize-space() = 'Télécharger les éléments sélectionnés']", chromedp.BySearch),
	); err != nil && !strings.Contains(err.Error(), "net::ERR_ABORTED") {
		return "", err
	}

	// This will block until the chromedp listener closes the channel
	guid := <-done

	out := filepath.Join(outputPath, guid)
	logger.Info("download complete", slog.String("guid", guid), slog.String("path", out))
	return out, nil
}

func unzipArchive(logger *slog.Logger, source string, destination string) error {
	logger = logger.With(slog.String("source", source), slog.String("destination", destination))

	logger.Info("opening zip file")
	reader, err := zip.OpenReader(source)
	if err != nil {
		return fmt.Errorf("failed to open zip file: %w", err)
	}
	defer reader.Close()

	for _, file := range reader.File {
		if err := unzipFile(logger, file, destination); err != nil {
			return err
		}
	}

	return nil
}

func unzipFile(logger *slog.Logger, file *zip.File, destination string) error {
	file.Name = strings.ToValidUTF8(file.Name, "") // Sanitize file name

	logger = logger.With(slog.String("file", file.Name))
	logger.Info("extracting file")

	if err := os.MkdirAll(filepath.Dir(filepath.Join(destination, file.Name)), os.ModePerm); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	outFile, err := os.OpenFile(filepath.Join(destination, file.Name), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer outFile.Close()

	inFile, err := file.Open()
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer inFile.Close()

	if _, err = io.Copy(outFile, inFile); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	return nil
}

func configureLogger(isDebug bool) *slog.Logger {
	var level = slog.LevelInfo
	if isDebug || os.Getenv("CHROMEDP_DEBUG") != "" {
		level = slog.LevelDebug
	}

	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	return slog.New(handler)
}

func removeDownloadedFiles(logger *slog.Logger, path string) error {
	logger = logger.With(slog.String("path", path))
	logger.Info("removing downloaded files")

	if err := os.RemoveAll(path); err != nil {
		return err
	}
	return nil
}
