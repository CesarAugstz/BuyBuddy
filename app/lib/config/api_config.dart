class ApiConfig {
  static const bool isDevelopment = bool.fromEnvironment('DEVELOPMENT', defaultValue: true);
  
  static const String devUrl = 'http://192.168.0.11:8080/api';
  static const String prodUrl = 'https://apibuybuddy.cgstz.xyz/api';
  
  static String get baseUrl => isDevelopment ? devUrl : prodUrl;
  static String get loginEndpoint => '$baseUrl/auth/login';
  static String get logoutEndpoint => '$baseUrl/auth/logout';
  static String get verifyTokenEndpoint => '$baseUrl/auth/verify';
}
