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
| `GHCR_PULL_USERNAME` | Yes | Không tìm thấy value thật; file example chỉ comment: `myorg` |
| `GHCR_PULL_TOKEN` | Yes | Chỉ thấy placeholder comment: `CHANGE_ME_GHCR_READ_TOKEN_OR_PAT` |

### Lưu ý bắt buộc cho production SSH/SCP

- `deploy-prod.yml` dùng cả `scp` và `ssh` tới `VPS_HOST:VPS_SSH_PORT`, nên host/port này phải reachable từ GitHub-hosted runner qua public internet.
- `VPS_SSH_PRIVATE_KEY` phải là full private key block, ví dụ bắt đầu bằng `-----BEGIN OPENSSH PRIVATE KEY-----` hoặc `-----BEGIN RSA PRIVATE KEY-----`.
- Không paste `.pub`, không paste dòng trong `authorized_keys`, không thêm dấu nháy, và không dùng key có passphrase nếu workflow chưa truyền thêm `passphrase`.
- Nếu VPS chỉ mở SSH trong mạng nội bộ, sau VPN, hoặc allowlist IP quá chặt, `scp-action` sẽ fail trước khi `release.sh` kịp chạy.

### Lưu ý bắt buộc cho production GHCR

- Hai package production `ghcr.io/leduytuanvu/avf-vending-api` và `ghcr.io/leduytuanvu/avf-vending-api-goose` hiện đang là `Private`.
- Vì vậy production deploy bắt buộc phải có cả `GHCR_PULL_USERNAME` và `GHCR_PULL_TOKEN`; không được dựa vào anonymous pull.
- `GHCR_PULL_TOKEN` nên là PAT hoặc token có ít nhất quyền `read:packages`.
- `GHCR_PULL_USERNAME` phải là GitHub username của account sở hữu token đó và account này phải thực sự có quyền đọc package private.

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
- `EMQX_API_KEY` / `EMQX_API_SECRET`: phải khớp với một EMQX REST API key đã được tạo sẵn trong EMQX và với `.env.production` thật
- `GHCR_PULL_USERNAME`: username GitHub của account có quyền đọc package private trên GHCR
- `GHCR_PULL_TOKEN`: PAT hoặc token có quyền `read:packages`

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
| `EMQX_API_KEY` | `<api-key-da-provision-truoc-trong-emqx>` |
| `EMQX_API_SECRET` | `<api-secret-tu-api-key-da-provision-truoc>` |
| `GHCR_PULL_USERNAME` | `<github-username-co-quyen-read-package>` |
| `GHCR_PULL_TOKEN` | `<PAT-hoac-token-co-read:packages>` |

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
- `Run Command Timeout` từ `appleboy/ssh-action` (thường rơi vào khoảng ~10 phút nếu workflow đang dùng default `command_timeout`)
  Đây là **timeout phía GitHub Actions** khi remote script chạy quá lâu, không nhất thiết có nghĩa VPS đã fail. Kiểm tra log phía VPS (`release.sh` / `docker compose`) xem deploy có đang tiếp tục chạy không, rồi rerun workflow sau khi đã tăng `command_timeout` trong workflow (repo hiện set `60m` cho staging/production SSH).

### Check nhanh khi production SSH / SCP fail

- `dial tcp <host>:<port>: i/o timeout`
  GitHub-hosted runner không mở được kết nối TCP tới `VPS_HOST:VPS_SSH_PORT`. Kiểm tra lại `VPS_HOST`, `VPS_SSH_PORT`, firewall VPS, cloud security group, router/NAT, và mọi allowlist IP ở phía server/provider.
- `missing production secret VPS_HOST` / `VPS_SSH_PORT` / `VPS_USER` / `VPS_SSH_PRIVATE_KEY`
  Secret production đang thiếu hoặc đang được đặt sai scope.
- `VPS_HOST must resolve from GitHub-hosted runners`
  `VPS_HOST` đang là hostname nội bộ, sai DNS public, hoặc chứa ký tự thừa / whitespace.
- `VPS_SSH_PRIVATE_KEY must contain the full private key PEM/OpenSSH block`
  Secret đang chứa public key, value bị cắt mất đầu/cuối, hoặc format private key không hợp lệ.
- `Run Command Timeout` từ `appleboy/ssh-action` / `appleboy/scp-action` (thường rơi vào khoảng ~10 phút nếu workflow đang dùng default `command_timeout`)
  Đây là **timeout phía GitHub Actions** khi remote script/transfer chạy quá lâu, không nhất thiết có nghĩa VPS đã fail. Kiểm tra log phía VPS (`release.sh deploy`, `docker compose`) xem deploy có đang tiếp tục chạy không, rồi rerun workflow sau khi đã tăng `command_timeout` trong workflow (repo hiện set `60m` cho production SSH/SCP).

### Check nhanh khi production GHCR login fail

- `Get "https://ghcr.io/v2/": denied: denied`
  `GHCR_PULL_USERNAME` / `GHCR_PULL_TOKEN` đang sai, token không có `read:packages`, hoặc account của token không có quyền đọc package private.
- `production deploy requires GHCR_PULL_USERNAME and GHCR_PULL_TOKEN because ghcr.io/leduytuanvu/avf-vending-api and ghcr.io/leduytuanvu/avf-vending-api-goose are private packages`
  Hai secret GHCR đang bị thiếu trong production environment/repository secrets.
- `set both GHCR_PULL_USERNAME and GHCR_PULL_TOKEN together for production GHCR pulls`
  Chỉ mới set một nửa cặp secret, nên workflow chặn trước khi remote deploy.
- Kiểm tra nhanh trên GitHub Packages UI:
  package phải hiện dưới đúng owner/repo, và account của token phải nhìn thấy package private đó.

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

### 3. Provision và đồng bộ `EMQX_API_KEY` / `EMQX_API_SECRET`

Tạo API key một lần trực tiếp trong EMQX, rồi copy cùng cặp giá trị đó vào `.env.production` và GitHub Secrets.

Ví dụ một cách làm an toàn:

1. SSH tunnel vào dashboard EMQX:

```bash
ssh -L 18083:127.0.0.1:18083 -p <VPS_SSH_PORT> <VPS_USER>@<VPS_HOST>
```

2. Mở dashboard EMQX tại `http://127.0.0.1:18083`
3. Vào `System > API Key`
4. Tạo một API key mới dành cho production deploy (một số bản EMQX Open Source không hiện dropdown role; cứ tạo key bình thường)
5. Lưu lại chính xác `EMQX_API_KEY` và `EMQX_API_SECRET`
6. Ghi cùng cặp giá trị đó vào `.env.production` và GitHub Secrets

Nếu bạn đã có sẵn key đang dùng, chỉ cần đọc lại từ `.env.production` thực:

```bash
cd <VPS_DEPLOY_PATH>/deployments/prod
grep -E '^(EMQX_API_KEY|EMQX_API_SECRET)=' .env.production
```

Lưu ý quan trọng về `.env.production`:

- Trong file **chỉ nên có đúng một dòng** `EMQX_API_KEY=...` và **đúng một dòng** `EMQX_API_SECRET=...`.
- Nếu có **trùng key** (ví dụ hai dòng `EMQX_API_KEY=`), shell `source ./.env.production` sẽ lấy **dòng cuối cùng**, dễ khiến bạn tưởng đã cập nhật nhưng thực tế vẫn đang dùng giá trị cũ.
- Kiểm tra nhanh có trùng hay không:

```bash
cd <VPS_DEPLOY_PATH>/deployments/prod
grep -nE '^EMQX_API_(KEY|SECRET)=' .env.production
```

Sau khi bạn đã validate trên VPS bằng `curl` (HTTP 200), bước tiếp theo là **cập nhật GitHub Secrets** `EMQX_API_KEY` / `EMQX_API_SECRET` cho đúng **cặp giá trị đang hoạt động** trong `.env.production`, rồi rerun workflow `Deploy Production`.

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
