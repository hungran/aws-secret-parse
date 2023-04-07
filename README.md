# aws-secret-parse

Retrive secret and generating application's config file for every format from AWS Secret Manager

## Usage & Prerequisite in k8s environment
- Configmap, AWS Secret Manager tag as input, output might be write to `emptyDir` in Pod
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