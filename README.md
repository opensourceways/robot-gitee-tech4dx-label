# Preparation

Note for this project `robot-gitee-tech4dx-label`.

## Step 1. Token for Gitee

Please modify the demo Gitee token in `config/gitee_token`.

## Step 2. Listen to org/repo

Please modify the `repos` in `config/detxh4_label.yaml`, the content should be `org/repo`.

## Step 3. Start the service

Make sure that be in the project path, and run the following command:

```sh
go run . --gitee-token-path=config/gitee_token --config-file=config/detxh4_label.yaml
```

## Other notice

The default port and router are `8888`, and `/gitee-hook separately`.

Be sure to run the project `robot-gitee-access` first. Otherwise no request can be sent to this project `robot-gitee-tech4dx-label` due to the proxy.
