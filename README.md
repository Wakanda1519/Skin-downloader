# 🎮 Skin downloader

**CLI-утилита для Rust Devblog комьюнити, которая декомпилирует Rust.Workshop.dll с помощью ILSpyCMD, парсит класс Approved, группирует SkinID по Shortname и скачивает весь скинпак с помощью SteamCMD**

![Go](https://img.shields.io/badge/Go-1.26%2B-00ADD8?style=flat-square&logo=go&logoColor=white)
![Windows](https://img.shields.io/badge/Windows-10%2B-0078D6?style=flat-square&logo=windows&logoColor=white)

<img width="1342" height="761" alt="oF7tUz0Myo7efHVmvRpEtG0vziZmC9zSF348XUj85Y6NwhMvKGnJS7H85cWaZBo0TXe3onKZLxmuV1khWfTYbwR2" src="https://github.com/user-attachments/assets/a1e49150-1794-4dbf-9380-cb3f1401520a" />

## Особенности

* Автоматический поиск `Rust.Workshop.dll` в рабочей папке и его декомпиляция через ILSpy CLI (`ilspycmd`).
* Автоматическая установка SteamCMD, если он не найден рядом или в PATH.
* Докачка прерванной загрузки: уже скачанные скины пропускаются.
* Параллельная загрузка в несколько воркеров SteamCMD и автоматические повторы ошибок.
* Итоговая сверка с диском: предложит докачать недостающие или удалить их из итогового json.

## Ограничения

* Утилита поддерживает только **Mono** клиент, где `Rust.Workshop.dll` хранится в открытом виде.
* Но и для **IL2Cpp** другой разработчик сделал решение - [ApprovedSkinExport](https://github.com/setfps/rust-ApprovedSkinExport), мод для выгрузки списка скинов из класса Approved в Rust.Workshop, работает через BepInEx.

## Установка

### Требования

* Windows 10+
* Go 1.26+
* ILSpyCND и SteamCMD отдельно ставить не нужно: утилита скачает их сама.
* `Rust.Workshop.dll` (в папке с игрой лежит по пути `...\RustClient_Data\Managed\Rust.Workshop.dll`).

### Сборка

```
go build -o Skindl.exe .
```

### Использование

1. Создайте папку, перенесите туда `Skindl.exe` и положите рядом `Rust.Workshop.dll` (из папки с игрой `...\RustClient_Data\Managed\`). Запустите.
2. `Skindl.exe` найдёт `Rust.Workshop.dll`, декомпилирует его через `ilspycmd` в папку `Rust.Workshop.csproj` и найдёт там `Approved.cs`.
3. Дальше программа сделает всё за вас: сформирует `Skins.json` и `Shortname.json`, затем спросит, начинать ли установку скинов, в конце предложит повторить установку скинов с ошибкой или удалить их из json.
4. Последний шаг предложит удалить лишние файлы: декомпилированный исходник `Rust.Workshop.csproj` вместе с `Rust.Workshop.dll`, SteamCMD и ILSpy CLI.

## Вопрос - ответ

<details>
  <summary><b>Почему именно Rust.Workshop.dll?</b></summary>
  <br>
  Именно в этой dll хранятся все SkinID, которые были доступны для данной версии девблога и которые 100% будут работать без проблем
</details>
<details>
  <summary><b>Для чего нужен Skins.json?</b></summary>
  <br>
  Он содержит сгруппированные SkinID по Shortname, что бы вы в 2 клика смогли их перенести в конфиг плагина.
</details>
<details>
  <summary><b>Для чего нужен Shortname.json?</b></summary>
  <br>
  Он содержит сгруппированные Shortname предметов, на которые есть скины, по категориям, что бы вы в 2 клика смогли их перенести в конфиг плагина.
</details>
<details>
  <summary><b>Зачем их скачивать, если они находятся в клиенте и работают?</b></summary>
  <br>
  Я проводил тесты на 196 Devblog (2017г.), где они так не работали, приходилось их скачивать, закидывать в папку workshop. А вот на 217 Devblog (2019г.) ситуация лучше, достаточно просто настроить плагин и они автоматически подхватываются из клиента. Поэтому ситуация здесь индвидуальная для каждого девблога.
</details>

## Лицензия

MIT License. Делайте с кодом что хотите.
