class Tagit < Formula
  desc "Daemon-first orchestrator for coding-agent CLIs (claude, codex, ...)"
  homepage "https://github.com/liliang-cn/tagit"
  version "0.1.0"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/liliang-cn/tagit/releases/download/v0.1.0/tagit_darwin_arm64.tar.gz"
      sha256 "315838f54ad836449a090d0f5a134250f3e941e07dc992fdf20e6655bd6f44ab"
    end
    on_intel do
      url "https://github.com/liliang-cn/tagit/releases/download/v0.1.0/tagit_darwin_amd64.tar.gz"
      sha256 "af752f3690c20a8cdebd7a7f61c3e0e040ec27a523dbf9b2ad989139e62d89cd"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/liliang-cn/tagit/releases/download/v0.1.0/tagit_linux_arm64.tar.gz"
      sha256 "7cd33f4fa716b169788b6da552e7dc64c9b86aca3bed4b386040f0ca6fbc74e3"
    end
    on_intel do
      url "https://github.com/liliang-cn/tagit/releases/download/v0.1.0/tagit_linux_amd64.tar.gz"
      sha256 "30a2a88f86285a7438c989f21608194797162124bd5fc8ecad23c28f91906b14"
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
