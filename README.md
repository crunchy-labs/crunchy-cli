# crunchy-cli

A [Rust](https://www.rust-lang.org/) written cli client for [Crunchyroll](https://www.crunchyroll.com).

<p align="center">
  <a href="https://github.com/crunchy-labs/crunchy-cli">
    <img src="https://img.shields.io/github/languages/code-size/crunchy-labs/crunchy-cli?style=flat-square" alt="Code size">
  </a>
  <a href="https://github.com/crunchy-labs/crunchy-cli/releases/latest">
    <img src="https://img.shields.io/github/downloads/crunchy-labs/crunchy-cli/total?style=flat-square" alt="Download Badge">
  </a>
  <a href="https://github.com/crunchy-labs/crunchy-cli/blob/master/LICENSE">
    <img src="https://img.shields.io/github/license/crunchy-labs/crunchy-cli?style=flat-square" alt="License">
  </a>
  <a href="https://github.com/crunchy-labs/crunchy-cli/releases">
    <img src="https://img.shields.io/github/v/release/crunchy-labs/crunchy-cli?include_prereleases&style=flat-square" alt="Release">
  </a>
  <a href="https://discord.gg/PXGPGpQxgk">
    <img src="https://img.shields.io/discord/994882878125121596?label=discord&style=flat-square" alt="Discord">
  </a>
  <a href="https://github.com/crunchy-labs/crunchy-cli/actions/workflows/ci.yml">
    <img src="https://img.shields.io/github/actions/workflow/status/crunchy-labs/crunchy-cli/ci.yml?branch=master&style=flat-square" alt="CI">
  </a>
</p>

<p align="center">
  <a href="#%EF%B8%8F-usage">Usage 🖥️</a>
  •
  <a href="#%EF%B8%8F-disclaimer">Disclaimer ☝️</a>
  •
  <a href="#-license">License ⚖</a>
</p>

> We are in no way affiliated with, maintained, authorized, sponsored, or officially associated with Crunchyroll LLC or any of its subsidiaries or affiliates.
> The official Crunchyroll website can be found at https://crunchyroll.com/.

> This README belongs to the _master_ branch which is currently under heavy development towards the next major version (3.0).
> It is mostly stable but some issues may still occur.
> If you do not want to use an under-development / pre-release version, head over to the _[golang](https://github.com/crunchy-labs/crunchy-cli/tree/golang)_ branch which contains the EOL but last stable version (and documentation for it).

## ✨ Features

- Download single videos and entire series from [Crunchyroll](https://www.crunchyroll.com).
- Archive episode or seasons in an `.mkv` file with multiple subtitles and audios.
- Specify a range which episodes to download from an anime.

## 💾 Get the executable

### 📥 Download the latest binaries

Checkout the [releases](https://github.com/crunchy-labs/crunchy-cli/releases) tab and get the binary from the newest (pre-)release.

### 🛠 Build it yourself

Since we do not support every platform and architecture you may have to build the project yourself.
This requires [git](https://git-scm.com/) and [Cargo](https://doc.rust-lang.org/cargo).
```shell
$ git clone https://github.com/crunchy-labs/crunchy-cli
$ cd crunchy-cli
$ cargo build --release
```
After the binary has built successfully it is available in `target/release`.

### Final steps

In order to make the binary globally accessible you will need to add it to `PATH` so it's recommended you move it to a general folder.


Examples:

- Linux/MacOS
  - ```shell
    mkdir ~/crunchy-cli
    mv /path/to/repo/target/release/crunchy-cli ~/crunchy-cli/crunchy # OR
    mv /path/to/downloaded/file/crunchy-cli(rest of filename here) ~/crunchy-cli/crunchy
    export PATH=$PATH:~/crunchy-cli
    ```
    
    For persistent usage you should add the above export to your `.shellrc`(.bashrc, .zshrc ,etc. file)
- Windows
  - Download the `.exe` file or build it yourself. Rename it to the way you will be calling it (ex: `crunchy.exe`) and move it into a folder where it's easily accessible. Afterwards follow a [guide](https://www.wikihow.com/Change-the-PATH-Environment-Variable-on-Windows) for adding that folder to the `PATH` variable. A restart of `CMD` or `powershell` might be required for the changes to take effect.

## 🖥️ Usage

> All shown command are just examples

Every command requires you to be logged in with an account.
It doesn't matter if this account is premium or not, both works (but as free user you do not have access to premium content).
You can pass your account via credentials (username & password) or refresh token.

- Refresh Token
  - To get the token you have to log in at [crunchyroll.com](https://www.crunchyroll.com/) and extract the `etp_rt` cookie. 
    The easiest way to get it is via a browser extension with lets you view your cookies, like [Cookie-Editor](https://cookie-editor.cgagnier.ca/) ([Firefox Store](https://addons.mozilla.org/en-US/firefox/addon/cookie-editor/); [Chrome Store](https://chrome.google.com/webstore/detail/cookie-editor/hlkenndednhfkekhgcdicdfddnkalmdm)).
    If installed, search the `etp_rt` entry and extract the value.
  - ```shell
    $ crunchy --etp-rt "abcd1234-zyxw-9876-98zy-a1b2c3d4e5f6"
    ```
- Credentials
  - Credentials must be provided as one single expression.
    Username and password must be separated by a `:`.
  - ```shell
    $ crunchy --credentials "user:password"
    ```
- Anonymous
  - Login without an account at all is also possible.
  - ```shell 
    $ crunchy --anonymous
    ```

### Login

If you do not want to provide your credentials every time you execute a command, they can be stored permanently on disk.
This can be done with the `login` subcommand.

```shell
$ crunchy --etp-rt "abcd1234-zyxw-9876-98zy-a1b2c3d4e5f6" login
```

Once set, you do not need to provide `--etp-rt` / `--credentials` anymore when using the cli.
This does not work if you've using this with `--anonymous`.

### Download

**Supported urls**
- Single episode
  ```shell
  $ crunchy download https://www.crunchyroll.com/watch/GRDQPM1ZY/alone-and-lonesome
  ```
- Episode range
  
  If you want only specific episodes / seasons of an anime you can easily provide the series url along with a _filter_.
  The filter has to be attached to the url. See the [wiki](https://github.com/crunchy-labs/crunchy-cli/wiki/Cli#filter) for more information
  ```shell
  $ crunchy download https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx[E1]
  ```
- Series
  ```shell
  $ crunchy download https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```
  
**Options**
- Audio language

  Which audio the episode(s) should be can be set via the `-a` / `--audio` flag.
  This only works if the url points to a series since episode urls are language specific.
  ```shell
  $ crunchy download -a de-DE https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```
  Default is your system language. If not supported by Crunchyroll, `en-US` (American English) is the default.

- Subtitle language

  Besides the audio, it's also possible to specify which language the subtitles should have with the `-s` / `--subtitle` flag.
  The subtitle will be hardsubbed (burned into the video) and thus, can't be turned off or on.
  ```shell
  $ crunchy download -s de-DE https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```
  Default is no subtitle.

- Output filename

  You can specify the name of the output file with the `-o` / `--output` flag.
  If you want to use any other file format than [`.ts`](https://en.wikipedia.org/wiki/MPEG_transport_stream) you need [ffmpeg](https://ffmpeg.org/).
  ```shell
  $ crunchy download -o "ditf.ts" https://www.crunchyroll.com/watch/GRDQPM1ZY/alone-and-lonesome
  ```
  Default is `{title}.ts`.

- Resolution

  The resolution for videos can be set via the `-r` / `--resolution` flag.
  ```shell
  $ crunchy download -r worst https://www.crunchyroll.com/watch/GRDQPM1ZY/alone-and-lonesome
  ```
  Default is `best`.

### Archive

**Supported urls**
- Series

  Only series urls are supported since single episode urls are (audio) language locked.
  ```shell
  $ crunchy archive https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```
  
**Options**
- Audio languages

  Which audios the episode(s) should be can be set via the `-a` / `--audio` flag.
  ```shell
  $ crunchy archive -a ja-JP -a de-DE https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```
  Can be used multiple times.
  Default is your system language (if not supported by Crunchyroll, `en-US` (American English) is the default) + `ja-JP` (Japanese).

- Subtitle languages

  Besides the audio, it's also possible to specify which languages the subtitles should have with the `-s` / `--subtitle` flag.
  ```shell
  $ crunchy archive -s de-DE https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```
  Default is all subtitles.

- Output filename

  You can specify the name of the output file with the `-o` / `--output` flag.
  The only supported file / container format is [`.mkv`](https://en.wikipedia.org/wiki/Matroska) since it stores / can store multiple audio, video and subtitle streams.
  ```shell
  $ crunchy archive -o "{title}.mkv" https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```
  Default is `{title}.mkv`.

- Resolution

  The resolution for videos can be set via the `-r` / `--resolution` flag.
  ```shell
  $ crunchy archive -r worst https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```
  Default is `best`.

- Merge behavior

  Because of local restrictions (or other reasons) some episodes with different languages does not have the same length (e.g. when some scenes were cut out).
  The ideal state, when multiple audios & subtitles used, would be if only one _video_ has to be stored and all other languages can be stored as audio-only.
  But, as said, this is not always the case.
  With the `-m` / `--merge` flag you can set what you want to do if some video lengths differ.
  Valid options are `audio` - store one video and all other languages as audio only; `video` - store the video + audio for every language; `auto` - detect if videos differ in length: if so, behave like `video` else like `audio`.
  Subtitles will always match to the first / primary audio and video.
  ```shell
  $ crunchy archive -m audio https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```
  Default is `auto`.

- Default subtitle

  `--default_subtitle` set which subtitle language should be set as default / auto appear when starting the downloaded video(s).
  ```shell
  $ crunchy archive --default_subtitle en-US https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```
  Default is none.

- No subtitle optimizations

  Subtitles, as Crunchyroll delivers them, look weird in some video players (#66). 
  This can be fixed by adding a specific entry to the subtitles.
  But since this entry is only a de-factor standard and not represented in the official specification of the subtitle format ([`.ass`](https://en.wikipedia.org/wiki/SubStation_Alpha)) it could cause issues with some video players (but no issue got reported so far, so it's relatively safe to use).
  `--no_subtitle_optimizations` can disable these optimizations.
  ```shell
  $ crunchy archive --no_subtitle_optimizations https://www.crunchyroll.com/series/GY8VEQ95Y/darling-in-the-franxx
  ```

# ☝️ Disclaimer

This tool is **ONLY** meant to be used for private purposes. To use this tool you need crunchyroll premium anyway, so there is no reason why rip and share the episodes.

**The responsibility for what happens to the downloaded videos lies entirely with the user who downloaded them.**

# ⚖ License

This project is licensed under the GNU General Public License v3.0 (GPL-3.0) - see the [LICENSE](LICENSE) file for more details.
