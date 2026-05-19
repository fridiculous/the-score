// swift-tools-version: 5.9

import PackageDescription

let package = Package(
    name: "ScoreMenu",
    platforms: [
        .macOS(.v13)
    ],
    products: [
        .executable(name: "ScoreMenu", targets: ["ScoreMenu"])
    ],
    targets: [
        .executableTarget(name: "ScoreMenu")
    ]
)
