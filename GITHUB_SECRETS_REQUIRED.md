# GitHub Secrets cần cấu hình

File này tổng hợp các GitHub Secrets / Inputs mà workflow hiện tại đang dùng.

## Production secrets đang được workflow dùng

| Tên biến | Bắt buộc | Value hiện tìm thấy |
|---|---|---|
| `VPS_HOST` | Yes | Không tìm thấy value thật trong repo/local files đã quét |
| `VPS_SSH_PORT` | Yes | Không tìm thấy value thật trong repo/local files đã quét |
| `VPS_USER` | Yes | Không tìm thấy value thật trong repo/local files đã quét |
| `VPS_SSH_PRIVATE_KEY` | Yes | Không tìm thấy value thật trong repo/local files đã quét |
| `VPS_DEPLOY_PATH` | Yes | Không tìm thấy value thật trong repo/local files đã quét |
| `EMQX_API_KEY` | Yes | Chỉ thấy placeholder: `CHANGE_ME_EMQX_API_KEY` |
| `EMQX_API_SECRET` | Yes | Chỉ thấy placeholder: `CHANGE_ME_LONG_RANDOM_EMQX_API_SECRET` |
| `GHCR_PULL_USERNAME` | Optional | Không tìm thấy value thật; file example chỉ comment: `myorg` |
| `GHCR_PULL_TOKEN` | Optional | Chỉ thấy placeholder comment: `CHANGE_ME_GHCR_READ_TOKEN_OR_PAT` |

## Staging secrets đang được workflow dùng

| Tên biến | Bắt buộc | Value hiện tìm thấy |
|---|---|---|
| `STAGING_HOST` | Yes | Không tìm thấy value thật trong repo/local files đã quét |
| `STAGING_PORT` | Yes | Không tìm thấy value thật trong repo/local files đã quét |
| `STAGING_USER` | Yes | Không tìm thấy value thật trong repo/local files đã quét |
| `STAGING_SSH_KEY` | Yes | Không tìm thấy value thật trong repo/local files đã quét |

### Staging deploy root hiện tại

- Workflow staging hiện dùng deploy root: `/opt/avf-vending-api`
- Thư mục staging đầy đủ được kỳ vọng là: `/opt/avf-vending-api/deployments/staging`
- Path này khác production root `/opt/avf-vending/avf-vending-api`, nên không nên copy nguyên path production sang staging

### Lưu ý bắt buộc cho staging GitHub Environment

- Bốn secret `STAGING_HOST`, `STAGING_PORT`, `STAGING_USER`, `STAGING_SSH_KEY` phải được tạo trong GitHub Environment tên `staging` vì workflow `deploy-staging.yml` chạy với `environment: staging`.
- `STAGING_SSH_KEY` phải là full private key block, ví dụ bắt đầu bằng `-----BEGIN OPENSSH PRIVATE KEY-----` hoặc `-----BEGIN RSA PRIVATE KEY-----`.
- Không paste `.pub`, không paste dòng trong `authorized_keys`, không thêm dấu nháy, và không dùng key có passphrase nếu workflow chưa truyền thêm `passphrase`.
- `STAGING_HOST` phải là IP hoặc hostname public resolve được từ GitHub-hosted runner. Nếu DNS nội bộ hoặc hostname gõ tay bị sai, SSH action sẽ fail trước khi script deploy chạy.

## Workflow inputs không phải secrets nhưng production deploy cần

Khi chạy `deploy-prod.yml` thủ công, bạn còn phải nhập:

- `release_tag`
- `app_image_ref`
- `goose_image_ref`
- `source_commit_sha` (optional nhưng nên nhập nếu image được build từ commit khác với commit hiện tại của workflow run)

## Điều tôi đã tìm thấy trong repo

### Trong `deployments/prod/.env.production.example`

```env
EMQX_API_KEY=CHANGE_ME_EMQX_API_KEY
EMQX_API_SECRET=CHANGE_ME_LONG_RANDOM_EMQX_API_SECRET
# GHCR_PULL_USERNAME=myorg
# GHCR_PULL_TOKEN=CHANGE_ME_GHCR_READ_TOKEN_OR_PAT
```

### Trong `deployments/staging/.env.staging.example`

```env
EMQX_API_KEY=CHANGE_ME_EMQX_API_KEY
EMQX_API_SECRET=CHANGE_ME_LONG_RANDOM_EMQX_API_SECRET
```

## Kết luận ngắn

- Repo hiện **không chứa value thật** cho các GitHub secrets production/staging bên trên.
- Bạn sẽ cần lấy value thật từ:
  - VPS/server hiện tại
  - tài khoản GHCR
  - file `.env.production` thực trên VPS
  - khóa SSH deploy đang dùng

## Gợi ý nơi lấy value thật

- `VPS_HOST`: IP/domain của VPS production
- `VPS_SSH_PORT`: cổng SSH thực tế trên VPS, thường là `22`
- `VPS_USER`: user deploy trên VPS, ví dụ `ubuntu`
- `VPS_SSH_PRIVATE_KEY`: private key tương ứng với public key đã add trên VPS
- `VPS_DEPLOY_PATH`: đường dẫn deploy root trên VPS, ví dụ `/opt/avf-vending/avf-vending-api`
- `EMQX_API_KEY` / `EMQX_API_SECRET`: phải khớp với `.env.production` thật và file bootstrap EMQX trên VPS
- `GHCR_PULL_USERNAME`: username/org dùng pull GHCR nếu image private
- `GHCR_PULL_TOKEN`: token có quyền `read:packages`

## Bảng Name / Value để copy-paste

Điền value thật vào cột `Value`, rồi paste vào GitHub Secrets UI.

### Production

| Name | Value |
|---|---|
| `VPS_HOST` | `<IP-hoac-domain-cua-VPS>` |
| `VPS_SSH_PORT` | `<vi-du-22>` |
| `VPS_USER` | `<vi-du-ubuntu>` |
| `VPS_SSH_PRIVATE_KEY` | `<toan-bo-private-key-PEM>` |
| `VPS_DEPLOY_PATH` | `</opt/avf-vending/avf-vending-api>` |
| `EMQX_API_KEY` | `<doc-tu-.env.production-thuc>` |
| `EMQX_API_SECRET` | `<doc-tu-.env.production-thuc>` |
| `GHCR_PULL_USERNAME` | `<username-hoac-org-pull-ghcr-neu-private>` |
| `GHCR_PULL_TOKEN` | `<token-read:packages-neu-private>` |

### Staging

| Name | Value |
|---|---|
| `STAGING_HOST` | `<IP-hoac-domain-cua-staging>` |
| `STAGING_PORT` | `<vi-du-22>` |
| `STAGING_USER` | `<user-ssh-staging>` |
| `STAGING_SSH_KEY` | `<toan-bo-private-key-PEM-cho-staging>` |

### Check nhanh khi staging SSH fail

- `ssh.ParsePrivateKey: ssh: no key found`
  `STAGING_SSH_KEY` đang rỗng, bị cắt mất đầu/cuối, là public key, hoặc là private key có format sai.
- `lookup <host> ... server misbehaving`
  `STAGING_HOST` không resolve được từ public DNS hoặc secret có ký tự thừa / whitespace.
- Workflow fail ngay trước `Deploy staging over SSH`
  Kiểm tra lại GitHub Environment `staging` có đủ cả 4 secret bắt buộc hay chưa.

## Template nhanh để paste vào chỗ khác

```text
VPS_HOST=
VPS_SSH_PORT=
VPS_USER=
VPS_SSH_PRIVATE_KEY=
VPS_DEPLOY_PATH=
EMQX_API_KEY=
EMQX_API_SECRET=
GHCR_PULL_USERNAME=
GHCR_PULL_TOKEN=
```

```text
STAGING_HOST=
STAGING_PORT=
STAGING_USER=
STAGING_SSH_KEY=
```

## Cách lấy từng value thật từ VPS hiện tại

### 1. SSH vào VPS

```bash
ssh -p <VPS_SSH_PORT> <VPS_USER>@<VPS_HOST>
```

### 2. Tìm `VPS_DEPLOY_PATH`

Nếu bạn đã biết repo đang nằm ở đâu thì dùng luôn. Nếu chưa rõ:

```bash
pwd
ls
```

Thông thường path sẽ là:

```text
/opt/avf-vending/avf-vending-api
```

### 3. Đọc `EMQX_API_KEY` và `EMQX_API_SECRET` từ `.env.production`

```bash
cd <VPS_DEPLOY_PATH>/deployments/prod
grep -E '^(EMQX_API_KEY|EMQX_API_SECRET)=' .env.production
```

### 4. Kiểm tra `GHCR_PULL_USERNAME` và `GHCR_PULL_TOKEN`

Nếu đang lưu trong `.env.production`:

```bash
grep -E '^(GHCR_PULL_USERNAME|GHCR_PULL_TOKEN)=' .env.production
```

Nếu không có kết quả:
- image có thể đang là public
- hoặc token đang được export thủ công ngoài shell
- hoặc secret chỉ đang nằm trong GitHub, không nằm trên VPS

### 5. Xác nhận `VPS_HOST`, `VPS_SSH_PORT`, `VPS_USER`

- `VPS_HOST`: chính là IP/domain bạn dùng để SSH
- `VPS_SSH_PORT`: cổng bạn dùng để SSH
- `VPS_USER`: user bạn dùng để SSH

Ví dụ:

```text
ssh -p 22 ubuntu@1.2.3.4
```

thì:
- `VPS_HOST=1.2.3.4`
- `VPS_SSH_PORT=22`
- `VPS_USER=ubuntu`

### 6. Lấy `VPS_SSH_PRIVATE_KEY`

Private key không thể đọc ngược từ server nếu bạn chỉ có public key trên VPS.

Bạn phải lấy từ máy đang dùng để deploy, ví dụ:

```powershell
type $env:USERPROFILE\.ssh\id_rsa
```

hoặc:

```powershell
type $env:USERPROFILE\.ssh\id_ed25519
```

Nếu bạn không biết key nào đúng, kiểm tra public key đang có trên VPS:

```bash
cat ~/.ssh/authorized_keys
```

rồi đối chiếu với:

```powershell
type $env:USERPROFILE\.ssh\id_rsa.pub
type $env:USERPROFILE\.ssh\id_ed25519.pub
```

### 7. Nếu cần lấy staging values

Làm tương tự production:
- SSH vào staging
- xác định host/port/user
- lấy private key từ máy local đang SSH được vào staging

## Lưu ý thực tế

- `VPS_SSH_PRIVATE_KEY` và `STAGING_SSH_KEY` phải paste nguyên block PEM, ví dụ bắt đầu bằng:

```text
-----BEGIN OPENSSH PRIVATE KEY-----
```

hoặc

```text
-----BEGIN RSA PRIVATE KEY-----
```

- Không thêm dấu nháy quanh value trong GitHub Secrets UI.
- `GHCR_PULL_USERNAME` / `GHCR_PULL_TOKEN` có thể để trống nếu GHCR package là public.

## Lưu ý

- `GITHUB_TOKEN` là built-in của GitHub Actions, không cần tự tạo secret thủ công.
- Không commit value thật của các secrets này vào repo.
