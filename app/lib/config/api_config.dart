class ApiConfig {
  static const String baseUrl = 'http://192.168.0.11:8080/api';
  static const String loginEndpoint = '$baseUrl/auth/login';
  static const String logoutEndpoint = '$baseUrl/auth/logout';
  static const String verifyTokenEndpoint = '$baseUrl/auth/verify';
}
