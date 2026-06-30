class Tagit < Formula
  desc "Daemon-first orchestrator for coding-agent CLIs (claude, codex, ...)"
  homepage "https://github.com/liliang-cn/tagit"
  version "0.1.0"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/liliang-cn/tagit/releases/download/v0.1.0/tagit_darwin_arm64.tar.gz"
      sha256 "e621fc698366985b90dd008b81e31da04f5552836d89b5e9c9ae46efc1f83edf"
    end
    on_intel do
      url "https://github.com/liliang-cn/tagit/releases/download/v0.1.0/tagit_darwin_amd64.tar.gz"
      sha256 "5013a57cf9303357ee5c4a31dc16f5b80982b54d5b5c9447bef9e24a205f65a0"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/liliang-cn/tagit/releases/download/v0.1.0/tagit_linux_arm64.tar.gz"
      sha256 "8712f2d90c0a2d6967a843a8bc7b13acc4942bda66365472e321b5c9694def71"
    end
    on_intel do
      url "https://github.com/liliang-cn/tagit/releases/download/v0.1.0/tagit_linux_amd64.tar.gz"
      sha256 "be7ee48aab05ffd435e1257af685c60ef98538ae73f37c173fb81e8d4c9d47c5"
    end
  end

  # Prebuilt binaries — no Go toolchain required.
  def install
    bin.install "tagit", "tagitd"
  end

  # `brew services start tagit` runs the daemon on login and keeps it alive.
  # The daemon uses ~/.tagit for state and config (agents.json, feishu.json, slack.json).
  service do
    run [opt_bin/"tagitd"]
    keep_alive true
    log_path var/"log/tagit.log"
    error_log_path var/"log/tagit.log"
  end

  test do
    assert_match "tagit usage", shell_output("#{bin}/tagit --help")
  end
end
