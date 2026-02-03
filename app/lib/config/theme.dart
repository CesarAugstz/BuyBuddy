import 'package:flutter/material.dart';

class AppTheme {
  static const Color primaryBlue = Color(0xFF1E3A8A);
  static const Color darkBlue = Color(0xFF1E40AF);
  static const Color lightGray = Color(0xFFF5F5F5);
  static const Color mediumGray = Color(0xFFE5E5E5);
  static const Color darkGray = Color(0xFF4B5563);
  static const Color nearBlack = Color(0xFF1F2937);
  
  static ThemeData get theme {
    return ThemeData(
      useMaterial3: true,
      colorScheme: ColorScheme.light(
        primary: primaryBlue,
        secondary: darkBlue,
        surface: Colors.white,
        onPrimary: Colors.white,
        onSecondary: Colors.white,
        onSurface: nearBlack,
      ),
      scaffoldBackgroundColor: lightGray,
      appBarTheme: AppBarTheme(
        backgroundColor: Colors.white,
        foregroundColor: nearBlack,
        elevation: 0,
        surfaceTintColor: Colors.transparent,
        iconTheme: IconThemeData(color: nearBlack),
      ),
      elevatedButtonTheme: ElevatedButtonThemeData(
        style: ElevatedButton.styleFrom(
          backgroundColor: primaryBlue,
          foregroundColor: Colors.white,
          elevation: 0,
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(8),
          ),
        ),
      ),
      drawerTheme: DrawerThemeData(
        backgroundColor: Colors.white,
      ),
    );
  }
}
