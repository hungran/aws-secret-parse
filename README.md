# aws-secret-parse

- Retrive secret and generating application's config file for every format from AWS Secret Manager
- Image using chainguard go image with 0 CVE

## Usage & Prerequisite in k8s environment
- Configmap, AWS Secret Manager tag key `env` and value as input, output is your file format you need (.env, appsetting.json)
- Can be use as initContainer...


## Eg

Input

```json
{
    "abc": "{{ .PerfectSecret }}",
    "xyz": "{{ .FooBar }}"
}
```

Output

```json
{
    "abc": "PerfectSecret_VALUE",
    "xyz": ""
}
```
## Idea
If you looking this with helm you will need
- [ ] Helm chart for creating configmap from template file
- [ ] InitContainer for creating outfile under `emptyDir`

If you interesting with this!

[!["Buy Me A Coffee"](https://www.buymeacoffee.com/assets/img/custom_images/orange_img.png)](https://www.buymeacoffee.com/hungran91)