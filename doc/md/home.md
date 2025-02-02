---
id: home
title: Atlas
slug: /
sidebar_position: 1
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';


![Atlas CLI in Action](https://release.ariga.io/images/assets/atlas-intro.gif)

## Installation

<Tabs
defaultValue="apple-intel"
values={[
{label: 'Apple Intel', value: 'apple-intel'},
{label: 'Apple Silicon', value: 'apple-silicon'},
{label: 'Linux', value: 'linux'},
{label: 'Windows', value: 'windows'},
]}>
<TabItem value="apple-intel">

Download latest release.
```shell
curl -LO https://release.ariga.io/atlas/atlas-darwin-amd64-v0.1.1
```

Make the atlas binary executable.
```shell
chmod +x ./atlas-darwin-amd64-v0.1.1
```

Move the atlas binary to a file location on your system PATH.
```shell
sudo mv ./atlas-darwin-amd64-v0.1.1 /usr/local/bin/atlas
```
```shell
sudo chown root: /usr/local/bin/atlas
```

</TabItem>
<TabItem value="apple-silicon">

Download latest release.
```shell
curl -LO https://release.ariga.io/atlas/atlas-darwin-arm64-v0.1.1
```

Make the atlas binary executable.
```shell
chmod +x ./atlas-darwin-arm64-v0.1.1
```

Move the atlas binary to a file location on your system PATH.
```shell
sudo mv ./atlas-darwin-arm64-v0.1.1 /usr/local/bin/atlas
```
```shell
sudo chown root: /usr/local/bin/atlas
```

</TabItem>
<TabItem value="linux">

Download latest release.
```shell
curl -LO https://release.ariga.io/atlas/atlas-linux-amd64-v0.1.1
```

Move the atlas binary to a file location on your system PATH.
```shell
sudo install -o root -g root -m 0755 ./atlas-linux-amd64-v0.1.1 /usr/local/bin/atlas
```

</TabItem>
<TabItem value="windows">

Download latest release.
```shell
curl -LO https://release.ariga.io/atlas/atlas-windows-amd64-v0.1.1.exe
```
Move the atlas binary to a file location on your system PATH.


</TabItem>
</Tabs>

## Schema Inspection

<Tabs
defaultValue="mysql"
values={[
{label: 'MySQL', value: 'mysql'},
{label: 'PostgreSQL', value: 'postgres'},
]}>
<TabItem value="mysql">

Inspect and save output to a schema file.
```shell
atlas schema inspect -d "mysql://root:pass@tcp(localhost:3306)/example" >> atlas.hcl
```

</TabItem>
<TabItem value="postgres">

Inspect and save output to a schema file.
```shell
atlas schema inspect -d "postgres://root:pass@0.0.0.0:5432/example?sslmode=disable" >> atlas.hcl
```

:::caution

sslmode disable is not recommended in prod.

:::

</TabItem>
</Tabs>

## Apply change to Schema

<Tabs
defaultValue="mysql"
values={[
{label: 'MySQL', value: 'mysql'},
{label: 'PostgreSQL', value: 'postgres'},
]}>
<TabItem value="mysql">

```shell
atlas schema apply -d "mysql://root:pass@tcp(localhost:3306)/example" -f atlas.hcl
```

</TabItem>
<TabItem value="postgres">

```shell
atlas schema apply -d "postgres://root:pass@0.0.0.0:5432/example?sslmode=disable" -f atlas.hcl
```

:::caution

sslmode disable is not recommended in prod.

:::

</TabItem>
</Tabs>

For more details and future plans read [Meet Atlas CLI](https://blog.ariga.io/meet-atlas-cli/).