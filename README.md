<p align="center"><strong>Version 2 is out 🥳, see all the <a href="https://github.com/ByteDream/crunchyroll-go/releases/tag/v2.0.0">changes.</a></strong></p>

# crunchyroll-go

A [Go](https://golang.org) library & cli for the undocumented [crunchyroll](https://www.crunchyroll.com) api. To use it, you need a crunchyroll premium account to for full (api) access.

<p align="center">
  <a href="https://github.com/ByteDream/crunchyroll-go">
    <img src="https://img.shields.io/github/languages/code-size/ByteDream/crunchyroll-go?style=flat-square" alt="Code size">
  </a>
  <a href="https://github.com/ByteDream/crunchyroll-go/releases/latest">
    <img src="https://img.shields.io/github/downloads/ByteDream/crunchyroll-go/total?style=flat-square" alt="Download Badge">
  </a>
  <a href="https://github.com/ByteDream/crunchyroll-go/blob/master/LICENSE">
    <img src="https://img.shields.io/github/license/ByteDream/crunchyroll-go?style=flat-square" alt="License">
  </a>
  <a href="https://golang.org">
    <img src="https://img.shields.io/github/go-mod/go-version/ByteDream/crunchyroll-go?style=flat-square" alt="Go version">
  </a>
  <a href="https://github.com/ByteDream/crunchyroll-go/releases/latest">
    <img src="https://img.shields.io/github/v/release/ByteDream/crunchyroll-go?style=flat-square" alt="Release">
  </a>
  <a href="https://discord.gg/gUWwekeNNg">
    <img src="https://img.shields.io/discord/915659846836162561?label=discord&style=flat-square" alt="Discord">
  </a>
</p>

<p align="center">
  <a href="#%EF%B8%8F-cli">CLI 🖥️</a>
  •
  <a href="#-library">Library 📚</a>
  •
  <a href="#%EF%B8%8F-disclaimer">Disclaimer ☝️</a>
  •
  <a href="#-license">License ⚖</a>
</p>

# 🖥️ CLI

## ✨ Features

- Download single videos and entire series from [crunchyroll](https://www.crunchyroll.com).
- Archive episode or seasons in an `.mkv` file with multiple subtitles and audios and compress them to gzip or zip files.
- Specify a range which episodes to download from an anime.

## 💾 Get the executable

- 📥 Download the latest binaries [here](https://github.com/ByteDream/crunchyroll-go/releases/latest) or get it from below:
  - [Linux (x64)](https://smartrelease.bytedream.org/github/ByteDream/crunchyroll-go/crunchy-{tag}_linux)
  - [Windows (x64)](https://smartrelease.bytedream.org/github/ByteDream/crunchyroll-go/crunchy-{tag}_windows.exe)
  - [MacOS (x64)](https://smartrelease.bytedream.org/github/ByteDream/crunchyroll-go/crunchy-{tag}_darwin)
- If you use Arch btw. or any other Linux distro which is based on Arch Linux, you can download the package via the [AUR](https://aur.archlinux.org/packages/crunchyroll-go/):
  ```
  $ yay -S crunchyroll-go
  ```
- 🛠 Build it yourself
  - use `make` (requires `go` to be installed):
  ```
  $ git clone https://github.com/ByteDream/crunchyroll-go
  $ cd crunchyroll-go
  $ make && sudo make install
  ```
  - use `go`:
  ```
  $ git clone https://github.com/ByteDream/crunchyroll-go
  $ cd crunchyroll-go/cmd/crunchyroll-go
  $ go build -o crunchy
  ```

## 📝 Examples

_Before reading_: Because of the huge functionality not all cases can be covered in the README.
Make sure to check the [wiki](https://github.com/ByteDream/crunchyroll-go/wiki/Cli), further usages and options are described there.

### Login

Before you can do something, you have to login first.

This can be performed via crunchyroll account email and password.
```
$ crunchy login user@example.com password
```

or via session id
```
$ crunchy login --session-id 8e9gs135defhga790dvrf2i0eris8gts
```

### Download

By default, the cli tries to download the episode with your system language as audio.
If no streams with your system language are available, the video will be downloaded with japanese audio and hardsubbed subtitles in your system language.
**If your system language is not supported, an error message will be displayed and en-US (american english) will be chosen as language.**

```
$ crunchy download https://www.crunchyroll.com/darling-in-the-franxx/episode-1-alone-and-lonesome-759575
```

With `-r best` the video(s) will have the best available resolution (mostly 1920x1080 / Full HD).
```
$ crunchy download -r best https://www.crunchyroll.com/darling-in-the-franxx/episode-1-alone-and-lonesome-759575
```

The file is by default saved as a `.ts` (mpeg transport stream) file.
`.ts` files may can't be played or are looking very weird (it depends on the video player you are using).
With the `-o` flag, you can change the name (and file ending) of the output file.
So if you want to save it as, for example, `mp4` file, just name it `whatever.mp4`.
**You need [ffmpeg](https://ffmpeg.org) to store the video in other file formats.**
```
$ crunchy download -o "daaaaaaaaaaaaaaaarling.ts" https://www.crunchyroll.com/darling-in-the-franxx/episode-1-alone-and-lonesome-759575
```

With the `--audio` flag you can specify which audio the video should have and with `--subtitle` which subtitle it should have.
Type `crunchy help download` to see all available locales.
```
$ crunchy download --audio ja-JP --subtitle de-DE https://www.crunchyroll.com/darling-in-the-franxx
```

##### Flags

The following flags can be (optional) passed to modify the [download](#download) process.

| Short | Extended       | Description                                                                    |
|-------|----------------|--------------------------------------------------------------------------------|
| `-a`  | `--audio`      | Forces audio of the video(s).                                                  |
| `-s`  | `--subtitle`   | Forces subtitle of the video(s).                                               |
| `-d`  | `--directory`  | Directory to download the video(s) to.                                         |
| `-o`  | `--output`     | Name of the output file.                                                       |
| `-r`  | `--resolution` | The resolution of the video(s). `best` for best resolution, `worst` for worst. |
| `-g`  | `--goroutines` | Sets how many parallel segment downloads should be used.                       |

### Archive

Archive works just like [download](#download). It downloads the given videos as `.mkv` files and stores all (soft) subtitles in it.
Default audio locales are japanese and your system language (if available) but you can set more or less with the `--language` flag.

Archive a file
```shell
$ crunchy archive https://www.crunchyroll.com/darling-in-the-franxx/darling-in-the-franxx/episode-1-alone-and-lonesome-759575
```

Downloads the first two episode of Darling in the FranXX and stores it compressed in a file.
```shell
$ crunchy archive -c "ditf.tar.gz" https://www.crunchyroll.com/darling-in-the-franxx/darling-in-the-franxx
```

##### Flags

The following flags can be (optional) passed to modify the [archive](#archive) process.

| Short | Extended       | Description                                                                                                                                                                                             |
|-------|----------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `-l`  | `--language`   | Audio locale which should be downloaded. Can be used multiple times.                                                                                                                                    |
| `-d`  | `--directory`  | Directory to download the video(s) to.                                                                                                                                                                  |
| `-o`  | `--output`     | Name of the output file.                                                                                                                                                                                |
| `-m`  | `--merge`      | Sets the behavior of the stream merging. Valid behaviors are 'auto', 'audio', 'video'. See the [wiki](https://github.com/ByteDream/crunchyroll-go/wiki/Cli#archive) for more information.               |
| `-c`  | `--compress`   | If is set, all output will be compresses into an archive. This flag sets the name of the compressed output file and the file ending specifies the compression algorithm (gzip, tar, zip are supported). |
| `-r`  | `--resolution` | The resolution of the video(s). `best` for best resolution, `worst` for worst.                                                                                                                          |
| `-g`  | `--goroutines` | Sets how many parallel segment downloads should be used.                                                                                                                                                |

### Help

- General help
  ```shell
  $ crunchy help
  ```
- Login help
  ```shell
  $ crunchy help login
  ```
- Download help
  ```shell
  $ crunchy help download
  ```

- Archive help
  ```shell
  $ crunchy help archive
  ```

### Global flags

These flags you can use across every sub-command

| Flag | Description                                          |
|------|------------------------------------------------------|
| `-q` | Disables all output.                                 |
| `-v` | Shows additional debug output.                       |
| `-p` | Use a proxy to hide your ip / redirect your traffic. |

# 📚 Library

Download the library via `go get`
```shell
$ go get github.com/ByteDream/crunchyroll-go
```

The documentation is available on [pkg.go.dev](https://pkg.go.dev/github.com/ByteDream/crunchyroll-go).

Examples how to use the library and some features of it are described in the [wiki](https://github.com/ByteDream/crunchyroll-go/wiki/Library).

# ☝️ Disclaimer

This tool is **ONLY** meant to be used for private purposes.
To use this tool you need crunchyroll premium anyway, so there is no reason why rip and share the episodes.

**The responsibility for what happens to the downloaded videos lies entirely with the user who downloaded them.**

# ⚖ License

This project is licensed under the GNU Lesser General Public License v3.0 (LGPL-3.0) - see the [LICENSE](LICENSE) file for more details.
