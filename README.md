# 🎮 Skin downloader

**CLI-утилита для Rust Devblog, которая парсит декомпилирует Rust.Workshop.dll с помощью ILSpyCMD, парсит класс Approved, группирует SkinID по Shortname и скачивает весь скинпак с помощью SteamCMD**

![Go](https://img.shields.io/badge/Go-1.26%2B-00ADD8?style=flat-square&logo=go&logoColor=white)
![Windows](https://img.shields.io/badge/Windows-10%2B-0078D6?style=flat-square&logo=windows&logoColor=white)

<img width="1342" height="761" alt="oF7tUz0Myo7efHVmvRpEtG0vziZmC9zSF348XUj85Y6NwhMvKGnJS7H85cWaZBo0TXe3onKZLxmuV1khWfTYbwR2" src="https://github.com/user-attachments/assets/a1e49150-1794-4dbf-9380-cb3f1401520a" />

## Особенности

* Автоматический поиск `Rust.Workshop.dll` в рабочей папке и его декомпиляция через ILSpy CLI (`ilspycmd`).
* Автоматическая установка SteamCMD, если он не найден рядом или в PATH.
* Докачка прерванной загрузки: уже скачанные скины пропускаются.
* Параллельная загрузка в несколько воркеров SteamCMD и автоматические повторы ошибок.
* Итоговая сверка с диском: предложит докачать недостающие или удалить их из итогового json.

> **Важно:** утилита поддерживает **Mono** клиент, не **il2cpp**, где `Rust.Workshop.dll` хранится в открытом виде. В dll хранится полный список SkinID, которые были доступны в Workshop на момент релиза девблога.

> **Зачем скачивать workshop:** На 196 Devblog надо скачивать скины и загружать в папку workshop, напрямую они не работают, на 217 Devblog достаточно просто заполнить конфиги плагина, и они подхватываются, поэтому для каждого девблога это индивидуально.

## Установка

### Требования

* Windows 10+
* Go 1.26+
* `Rust.Workshop.dll` (в папке с игрой лежит по пути `...\RustClient_Data\Managed\Rust.Workshop.dll`).
* .NET SDK — нужен для `ilspycmd`. Если его нет, утилита попробует установить `ilspycmd` сама через.
* SteamCMD отдельно ставить не нужно: утилита скачает его сама.

### Сборка

```
go build -o Skindl.exe .
```

### Использование

1. Создайте папку, перенесите туда `Skindl.exe` и положите рядом `Rust.Workshop.dll` (из папки с игрой `...\RustClient_Data\Managed\`). Запустите.
2. `Skindl.exe` найдёт `Rust.Workshop.dll`, декомпилирует его через `ilspycmd` в папку `Rust.Workshop.csproj` и найдёт там `Approved.cs`.
3. Дальше программа сделает всё за вас: сформирует `Skins.json` и `Shortname.json`, затем спросит, начинать ли установку скинов, в конце предложит повторить установку скинов с ошибкой или удалить их из json.
4. Последний шаг предложит удалить лишние файлы: декомпилированный исходник `Rust.Workshop.csproj` вместе с `Rust.Workshop.dll`, SteamCMD и ILSpy CLI.

## Что делать с полученными файлами?

* `Skins.json` можно перенести в список скинов вашего плагина, `Shortname.json` можно перенести в конфиг вашего плагина. Json сформирован на примере плагина XSkinMenu.cs
* Папку `Workshop` разместите в директории вашего девблога, если он не умеет напрямую брать их из файлов игры.
