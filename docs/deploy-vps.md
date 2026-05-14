# Triển khai Hermes Tunnel trên VPS với HTTPS

Tài liệu này mô tả cách biến máy cá nhân thành server tự host thông qua VPS public, domain thật, DNS wildcard và HTTPS bằng Caddy + Let's Encrypt.

## 1. Kiến trúc

```text
Internet
  -> https://app.tunnel.example.com
  -> Caddy trên VPS (:443)
  -> Hermes gateway nội bộ (127.0.0.1:8080)
  -> control TCP tới Hermes client (:8081)
  -> http://localhost:3000 trên máy cá nhân
```

Hermes client chủ động kết nối ra VPS, nên máy cá nhân không cần mở port router.

## 2. DNS Cloudflare

Thay `example.com` bằng domain thật và `<VPS_IP>` bằng IP public của VPS.

```text
A  tunnel.example.com    <VPS_IP>
A  *.tunnel.example.com  <VPS_IP>
```

Để cấp wildcard certificate, tạo Cloudflare API token với quyền tối thiểu:

- Zone:Read cho domain.
- DNS:Edit cho domain.

## 3. Cài Hermes trên VPS

Ví dụ dưới đây dùng Ubuntu/Debian. Với hệ điều hành khác, dùng lệnh package manager tương đương.

```bash
sudo apt update
sudo apt install -y git golang-go
git clone <YOUR_REPO_URL> hermes-tunnel
cd hermes-tunnel
go build -o hermes ./cmd/hermes
sudo install -m 0755 hermes /usr/local/bin/hermes
```

Tạo token mạnh:

```bash
openssl rand -base64 32
```

Tạo env cho Hermes:

```bash
sudo useradd --system --user-group --home /var/lib/hermes --shell /usr/sbin/nologin hermes
sudo mkdir -p /etc/hermes
sudo cp deploy/hermes.env.example /etc/hermes/hermes.env
sudo nano /etc/hermes/hermes.env
sudo chmod 640 /etc/hermes/hermes.env
```

Copy systemd service:

```bash
sudo cp deploy/hermes.service.example /etc/systemd/system/hermes.service
sudo systemctl daemon-reload
sudo systemctl enable --now hermes
sudo systemctl status hermes
```

Kiểm tra gateway nội bộ:

```bash
curl http://127.0.0.1:8080/__hermes/health
```

## 4. Cài Caddy với Cloudflare DNS plugin

Cần bản Caddy có plugin `github.com/caddy-dns/cloudflare` để cấp wildcard certificate qua DNS-01.

```bash
sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https curl
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list
sudo apt update
sudo apt install -y caddy

go install github.com/caddyserver/xcaddy/cmd/xcaddy@latest
~/go/bin/xcaddy build --with github.com/caddy-dns/cloudflare
sudo mv caddy /usr/bin/caddy
sudo setcap cap_net_bind_service=+ep /usr/bin/caddy
```

Tạo env cho Caddy:

```bash
sudo mkdir -p /etc/caddy
sudo cp deploy/caddy.env.example /etc/caddy/cloudflare.env
sudo nano /etc/caddy/cloudflare.env
```

Cho service Caddy đọc env:

```bash
sudo systemctl edit caddy
```

Nội dung override:

```ini
[Service]
EnvironmentFile=/etc/caddy/cloudflare.env
```

Copy Caddyfile mẫu rồi đổi `example.com` thành domain thật:

```bash
sudo cp deploy/Caddyfile.example /etc/caddy/Caddyfile
sudo nano /etc/caddy/Caddyfile
sudo systemctl daemon-reload
sudo systemctl restart caddy
sudo systemctl status caddy
```

## 5. Firewall

Mở các port cần thiết:

```bash
sudo ufw allow OpenSSH
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw allow 8081/tcp
sudo ufw enable
sudo ufw status
```

Không mở `8080/tcp` ra Internet. Hermes public HTTP chỉ nên bind `127.0.0.1:8080`, để Caddy proxy nội bộ.

## 6. Kết nối máy cá nhân

Trên máy đang chạy app local ở `localhost:3000`:

```powershell
hermes connect --name app --local http://localhost:3000 --server <VPS_IP>:8081 --token <strong-token>
```

Sau đó truy cập từ Internet:

```text
https://app.tunnel.example.com
```

Nếu tắt client local, gateway sẽ trả lỗi tunnel chưa kết nối.

## 7. Troubleshooting

- `NXDOMAIN`: kiểm tra DNS wildcard `*.tunnel.example.com`.
- Caddy không lấy được cert: kiểm tra Cloudflare API token và log `journalctl -u caddy -f`.
- `502 Bad Gateway`: kiểm tra `systemctl status hermes` và `curl http://127.0.0.1:8080/__hermes/health`.
- Client không kết nối được: kiểm tra firewall VPS đã mở `8081/tcp`, token khớp và command `hermes server` đang chạy.
