# Hermes Tunnel

Hermes Tunnel là một CLI app viết bằng Go giúp đưa dịch vụ HTTP đang chạy trên máy cá nhân, ví dụ `localhost:3000`, ra ngoài thông qua một gateway công khai. Ý tưởng giống một đường hầm ngược: client trên máy cá nhân chủ động kết nối ra server, server nhận request public rồi chuyển ngược về localhost.

## Tính năng hiện tại

- `hermes server`: chạy gateway public và cổng control cho client.
- `hermes connect`: đăng ký một dịch vụ local theo tên tunnel.
- Hỗ trợ route dạng `/{tunnel-name}/path`, ví dụ `/app/users`.
- Hỗ trợ subdomain local dạng `app.localhost`.
- Protocol MVP dùng JSON newline-delimited qua TCP.

## Cấu trúc dự án

```text
cmd/hermes/          entrypoint CLI
internal/cli/        khai báo command Cobra
internal/client/     client kết nối từ máy local đến gateway
internal/gateway/    public gateway và control server
internal/protocol/   message, header, body limit dùng chung
internal/routing/    phân tích tunnel name từ path/domain
.air.toml            cấu hình live reload cho Air
```

## Yêu cầu

- Go 1.22 trở lên
- Air nếu muốn live reload khi phát triển

Kiểm tra Go:

```powershell
go version
```

Cài dependency:

```powershell
go mod tidy
```

## Cách sử dụng CLI

Chạy gateway:

```powershell
go run ./cmd/hermes server --public :8080 --control :8081 --token dev-secret
```

Mở terminal khác, kết nối app local:

```powershell
go run ./cmd/hermes connect --name app --local http://localhost:3000 --server 127.0.0.1:8081 --token dev-secret
```

Truy cập tunnel:

```powershell
curl http://localhost:8080/app/
```

Hoặc dùng subdomain local:

```powershell
curl http://app.localhost:8080/
```

Xem version:

```powershell
go run ./cmd/hermes version
```

## Các flag chính

### `server`

- `--public`: địa chỉ HTTP public, mặc định `:8080`.
- `--control`: địa chỉ TCP cho client tunnel, mặc định `:8081`.
- `--token`: token dùng chung để client được phép kết nối.
- `--max-body-bytes`: giới hạn dung lượng request/response.

### `connect`

- `--name`: tên tunnel public, ví dụ `app`. Bắt buộc.
- `--local`: URL service local, ví dụ `http://localhost:3000`.
- `--server`: địa chỉ control server, ví dụ `127.0.0.1:8081`.
- `--token`: token khớp với server.

## Phát triển với Air

Nếu chạy `air` bị lỗi:

```text
air: The term 'air' is not recognized...
```

Nghĩa là Windows chưa tìm thấy executable `air` trong `PATH`, thường do Air chưa được cài hoặc thư mục `GOPATH/bin` chưa nằm trong biến môi trường `PATH`.

Cài Air:

```powershell
go install github.com/air-verse/air@latest
```

Kiểm tra nơi Go cài binary:

```powershell
go env GOPATH
```

Sau đó thêm thư mục `bin` tương ứng vào `PATH`. Ví dụ nếu `GOPATH` là `C:\Users\luan\go`, hãy thêm:

```text
C:\Users\luan\go\bin
```

Đóng mở lại PowerShell rồi kiểm tra:

```powershell
air -v
```

Chạy live reload:

```powershell
air
```

Truyền tham số cho CLI qua Air:

```powershell
air -- server --public :8080 --control :8081 --token dev-secret
```

## Kiểm thử và build

```powershell
go test ./...
go build ./cmd/hermes
```

## Hướng phát triển tiếp theo

- Thêm TLS cho control channel.
- Chuyển protocol sang streaming frame để hỗ trợ upload lớn.
- Thêm quản lý account, tunnel metadata và token bền vững.
- Hỗ trợ domain thật và tự động cấp chứng chỉ.
- Đóng gói release binary cho Windows, Linux và macOS.
