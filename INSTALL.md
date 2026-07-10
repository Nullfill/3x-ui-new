# نصب 3X-UI Multiplier

این fork بر پایه 3x-ui نسخه `v3.4.2` است و قابلیت محاسبه ضریب ترافیک فقط برای مصرف آینده را اضافه می‌کند.

## نصب

روی Ubuntu 20.04+ یا Debian 11+ با کاربر root اجرا کنید:

```bash
bash <(curl -Ls https://raw.githubusercontent.com/Nullfill/3x-ui-new/main/install.sh)
```

نسخه پیش‌فرض `v3.4.2-multiplier-1` است. برای انتخاب نسخه دیگر:

```bash
VERSION=v3.4.2-multiplier-2 bash <(curl -Ls https://raw.githubusercontent.com/Nullfill/3x-ui-new/main/install.sh)
```

## آپدیت

```bash
bash <(curl -Ls https://raw.githubusercontent.com/Nullfill/3x-ui-new/main/update.sh)
```

قبل از جایگزینی، از باینری و دیتابیس موجود بکاپ timestampدار گرفته می‌شود. دانلود release با SHA256 منتشرشده بررسی می‌شود.

## حذف

```bash
bash <(curl -Ls https://raw.githubusercontent.com/Nullfill/3x-ui-new/main/uninstall.sh)
```

اسکریپت حذف، سرویس و فایل‌های برنامه را پاک می‌کند؛ دیتابیس `/etc/x-ui/x-ui.db` بدون تأیید صریح کاربر حذف نمی‌شود.

## مسیرها

- Binary: `/usr/local/x-ui/x-ui`
- Database: `/etc/x-ui/x-ui.db`
- Service: `/etc/systemd/system/x-ui.service`
- Release repository: `Nullfill/3x-ui-new`
