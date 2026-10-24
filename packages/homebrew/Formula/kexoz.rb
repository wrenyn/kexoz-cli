class Kexoz < Formula
  desc "Instant localhost tunnels for developers — ngrok alternative"
  homepage "https://kexoz.dev"
  version "1.0.6"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/wrenyn/kexoz-cli/releases/download/v1.0.6/kexoz-darwin-arm64"
      sha256 "PLACEHOLDER"
    end
    on_intel do
      url "https://github.com/wrenyn/kexoz-cli/releases/download/v1.0.6/kexoz-darwin-amd64"
      sha256 "PLACEHOLDER"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/wrenyn/kexoz-cli/releases/download/v1.0.6/kexoz-linux-arm64"
      sha256 "PLACEHOLDER"
    end
    on_intel do
      url "https://github.com/wrenyn/kexoz-cli/releases/download/v1.0.6/kexoz-linux-amd64"
      sha256 "PLACEHOLDER"
    end
  end

  def install
    if OS.mac?
      arch = Hardware::CPU.arm? ? "arm64" : "amd64"
      bin.install "kexoz-darwin-#{arch}" => "kexoz"
    else
      arch = Hardware::CPU.arm? ? "arm64" : "amd64"
      bin.install "kexoz-linux-#{arch}" => "kexoz"
    end
  end

  test do
    system "#{bin}/kexoz", "--version"
  end
end
