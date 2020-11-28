## Localization

### Translation of new strings
- Find new strings to translate:
    ```bash
    goi18n extract -outdir locales
    goi18n merge -outdir locales locales/active.*.toml
    ```
    You will get "translate.\<language code\>.toml" files with strings that translations need to be updated.
    
- Edit translations files
> You need to replace the special line break character from `\\n` to `\n`

- Merge the translations into active files:
    ```bash
    goi18n merge -outdir locales locales/active.*.toml locales/translate.*.toml
    ```

- (Optional) Remove "translate.*.toml" files:
    ```bash
    rm locales/translate.*.toml
    ```

### Translation to a new language
- Generate a file with all strings to translate:
    ```bash
    goi18n extract -outdir locales -sourceLanguage <language code>
    ```

- Edit the translations file

- Merge translations (this command will append hash codes of the strings):
    ```bash
    goi18n merge -outdir locales locales/active.*.toml
    ```
