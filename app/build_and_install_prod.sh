#!/bin/bash

set -e

echo "🚀 Building BuyBuddy Production APK..."

flutter clean
flutter pub get

flutter build apk --release --dart-define=DEVELOPMENT=false

APK_PATH="build/app/outputs/flutter-apk/app-release.apk"

if [ ! -f "$APK_PATH" ]; then
    echo "❌ Build failed: APK not found at $APK_PATH"
    exit 1
fi

echo "✅ Build successful!"
echo "📦 APK size: $(du -h $APK_PATH | cut -f1)"

if command -v adb &> /dev/null; then
    DEVICE_COUNT=$(adb devices | grep -v "List" | grep "device$" | wc -l)
    
    if [ $DEVICE_COUNT -eq 0 ]; then
        echo "⚠️  No Android device connected"
        echo "📍 APK location: $APK_PATH"
        exit 0
    fi
    
    echo "📱 Installing on connected device..."
    adb install -r "$APK_PATH"
    
    echo "✅ Installation complete!"
    echo "🎉 BuyBuddy is ready to use!"
else
    echo "⚠️  ADB not found"
    echo "📍 APK location: $APK_PATH"
    echo "💡 Transfer the APK to your device manually"
fi
