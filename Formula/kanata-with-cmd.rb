class KanataWithCmd < Formula
  desc "Cross-platform keyboard remapper with command actions enabled"
  homepage "https://github.com/jtroo/kanata"
  url "https://github.com/4evy/kanata/archive/c8c720ded5a34bbc4bdfbfbe33c97b7bb2e60e77.tar.gz"
  version "1.12.0-prerelease-2"
  sha256 "b1dbe6e3dd3dd37fa50cdd69a8b0693fbc967a607776d40cf07b784da0a3d3af"
  license "LGPL-3.0-only"
  head "https://github.com/4evy/kanata.git", branch: "main"

  depends_on "rust" => :build

  conflicts_with "kanata", because: "both install a kanata binary"

  def install
    system "cargo", "install", "--features", "cmd", *std_cargo_args
  end

  test do
    (testpath/"kanata.kbd").write <<~LISP
      (defsrc
        caps
      )

      (deflayer base
        caps
      )
    LISP

    system bin/"kanata", "--check", "--cfg", testpath/"kanata.kbd", "--no-wait"
  end
end
