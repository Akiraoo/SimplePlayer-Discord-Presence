# Simple Player Discord Presence

把**電腦版 Simple Player 網頁**正在播放的歌曲顯示在 Discord 狀態（「正在聆聽」）。

搭配 [Simple Player Web Server](https://github.com/Akiraoo/SimplePlayer-Web-Server) 使用（需要有 Discord 狀態事件的新版網頁）。

* 直接推到本機的 Discord 桌面 App，**不用登入 Discord**。
* 只支援電腦版網頁；Android App 和手機版網頁不會顯示。
* 暫停時保留狀態並標示 ⏸，進度條停在暫停的位置；關掉分頁或關掉開關時狀態會自動清除。

由兩個部分組成：

| 部分 | 作用 |
| ---- | ---- |
| `SimplePlayerPresence.exe` | 在背景執行的小程式（隨 Windows 自動啟動），負責和 Discord App 溝通 |
| Chrome 擴充功能 | 讀取網頁正在播放的歌曲，交給背景程式；工具列圖示可以開關 |

瀏覽器擴充功能不能直接連到 Discord App，所以需要背景程式當橋樑。

## 安裝（Windows）

1. 從 [Releases](../../releases) 下載並執行 `SimplePlayerPresence.exe`。
   * 它會把自己安裝到 `%LOCALAPPDATA%\SimplePlayerPresence`，設定成開機自動在背景執行（只寫入目前使用者的設定，不需要系統管理員）。
   * 程式沒有數位簽章，Windows 可能跳出「Windows 已保護您的電腦」，按「其他資訊 → 仍要執行」。
2. 安裝完成後會自動打開 Chrome 的擴充功能頁面和擴充功能資料夾：開啟右上角「開發人員模式」，按「載入未封裝項目」，選擇打開的那個 `extension` 資料夾。

   Chrome 不允許程式自動安裝不是從 Chrome 線上應用程式商店下載的擴充功能，所以這一步要手動做一次。
3. 開啟 Discord App，在電腦版 Simple Player 網頁播放歌曲。

按工具列上的擴充功能圖示可以開關，並查看背景程式和 Discord 的連線狀態。

### 解除安裝

再執行一次 `SimplePlayerPresence.exe`（或 `%LOCALAPPDATA%\SimplePlayerPresence\SimplePlayerPresence.exe`），選「是」解除安裝，再到 `chrome://extensions` 移除擴充功能。

## 封面和按鈕

Discord 需要從網路上抓封面圖片，所以只有在用 **https 公開網址**（例如設定了 `publicOrigin` 並透過反向代理）開啟網頁時，才會顯示歌曲封面和「在 Simple Player 收聽」按鈕。用區網 IP 或 localhost 開啟時只會顯示曲名和歌手。

## Discord Application ID

預設使用 Simple Player 的 Application ID，Discord 上會顯示「正在聆聽 Simple Player」。想換成自己的名稱，可以到 [Discord Developer Portal](https://discord.com/developers/applications) 建立一個 Application，把它的 Application ID 填到擴充功能的「進階設定」。

## 自己編譯

背景程式的原始碼在 `app/`（Go，沒有外部套件）。安裝 [Go](https://go.dev/dl/) 後執行 `app\build.bat`，會產生 `SimplePlayerPresence.exe`（擴充功能會一起打包進 exe 裡）。

背景程式只監聽 `127.0.0.1:47823`，而且只接受擴充功能送來的請求，其他網頁無法使用。

## License

Apache License 2.0，見 `LICENSE`。
