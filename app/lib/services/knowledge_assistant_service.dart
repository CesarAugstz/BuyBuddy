import 'shopping_assistant_service.dart';

class KnowledgeAssistantService extends ShoppingAssistantService {
  KnowledgeAssistantService({super.authService, super.client})
    : super(assistantPath: '/knowledge/assistant');
}
