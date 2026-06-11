package i18n

// Brazilian Portuguese dictionary for the engine's own UI strings. Shipped
// built in because the engine and its first applications serve PT-BR users;
// applications extend it via Register.
func init() {
	Register("pt-BR", map[string]string{
		// error pages
		"Page not found": "Página não encontrada",
		"Access denied":  "Acesso negado",
		"Bad request":    "Requisição inválida",
		"Internal error": "Erro interno",
		"The address you accessed does not exist or was removed.":       "O endereço acessado não existe ou foi removido.",
		"You do not have permission to access this page.":               "Você não tem permissão para acessar esta página.",
		"The server could not understand the request.":                  "A solicitação não pôde ser entendida pelo servidor.",
		"Something went wrong while processing the request. Try again.": "Algo deu errado ao processar a solicitação. Tente novamente.",
		"Reference code": "Código de referência",
		"Back to home":   "Voltar ao início",

		// auth
		"Enter username and password.":                                  "Informe usuário e senha.",
		"Enter a username and password.":                                "Informe nome de usuário e senha.",
		"Invalid credentials.":                                          "Credenciais inválidas.",
		"Too many attempts. Wait a moment and try again.":               "Muitas tentativas. Aguarde um instante e tente novamente.",
		"Passwords do not match.":                                       "As senhas não conferem.",
		"Invite link is invalid or expired.":                            "Link de convite inválido ou expirado.",
		"Could not create the user. The username may already be taken.": "Não foi possível criar o usuário. O nome de usuário pode já estar em uso.",
		"Enter an email.":                                               "Informe um email.",
		"User updated.":                                                 "Usuário atualizado.",
		"Sign in":                                                       "Entrar",
		"Username":                                                      "Usuário",
		"Password":                                                      "Senha",
		"Confirm password":                                              "Confirme a senha",
		"Create account":                                                "Criar conta",
		"Invite-only access. Ask the administrator for a link.": "Acesso somente por convite. Solicite um link ao administrador.",
		"username must be between 3 and 30 characters":          "o nome de usuário deve ter entre 3 e 30 caracteres",
		"username contains invalid characters":                  "o nome de usuário contém caracteres inválidos",

		// runtime forms
		"required field":                              "campo obrigatório",
		"invalid value":                               "valor inválido",
		"required field: %s":                          "campo obrigatório: %s",
		"Required field: %s":                          "Campo obrigatório: %s",
		"Invalid value for %s":                        "Valor inválido para %s",
		"Value already exists for field %s":           "O valor já existe para o campo %s",
		"Field %s exceeds the limit of %d characters": "Campo %s excede o limite de %d caracteres",
		"Could not start the transaction":             "Erro ao iniciar transação",
		"Could not finish saving (ref %s)":            "Erro ao finalizar (ref %s)",
		"Record created successfully":                 "Registro criado com sucesso",
		"Record updated successfully":                 "Registro atualizado com sucesso",
		"Record deleted successfully":                 "Registro excluído com sucesso",
		"Record not found":                            "Registro não encontrado",
		"Could not create the record":                 "Erro ao criar registro",
		"The record was modified by another user. Review the data before saving again.": "Registro foi modificado por outro usuário. Revise os dados antes de salvar novamente.",
		"Conflict: the record was modified by another user. Reload the page.":           "Conflito: registro foi modificado por outro usuário. Recarregue a página.",
		"Error: invalid revision": "Erro: rev inválida",
		"Script error: %s":        "Erro no script: %s",

		// admin: forms
		"Form created successfully":             "Formulário criado com sucesso",
		"Form updated successfully":             "Formulário atualizado com sucesso",
		"Form deleted successfully":             "Formulário excluído com sucesso",
		"Form has no linked EAV table":          "Formulário não possui tabela EAV vinculada",
		"EAV table not found":                   "Tabela EAV não encontrada",
		"Name and label are required":           "Nome e Label são obrigatórios",
		"Element name is required":              "Nome do elemento é obrigatório",
		"Element added":                         "Elemento adicionado",
		"Element updated":                       "Elemento atualizado",
		"Element removed":                       "Elemento removido",
		"Could not create the form (ref %s)":    "Erro ao criar formulário (ref %s)",
		"Could not create the element (ref %s)": "Erro ao criar elemento (ref %s)",
		"Could not update (ref %s)":             "Erro ao atualizar (ref %s)",
		"Could not update":                      "Erro ao atualizar",
		"Could not delete (ref %s)":             "Erro ao excluir (ref %s)",

		// admin: EAV schema and records
		"Table updated successfully":                  "Tabela atualizada com sucesso",
		"Could not create the EAV table (ref %s)":     "Erro ao criar tabela EAV (ref %s)",
		"Could not update the table (ref %s)":         "Erro ao atualizar tabela (ref %s)",
		"Name is required":                            "Nome é obrigatório",
		"Machine name is required":                    "Nome da máquina é obrigatório",
		"Name must be at most 100 characters":         "Nome deve ter no máximo 100 caracteres",
		"Machine name must be at most 100 characters": "Nome da máquina deve ter no máximo 100 caracteres",
		"Description must be at most 500 characters":  "Descrição deve ter no máximo 500 caracteres",
		"Machine name must contain only lowercase letters, numbers and underscores, and start with a letter": "Nome da máquina deve conter apenas letras minúsculas, números e underscores, e começar com letra",
		"Name and machine name are required":        "Nome e Nome da Máquina são obrigatórios",
		"Machine name, label and type are required": "Nome da máquina, rótulo e tipo são obrigatórios",
		"Attribute created successfully":            "Atributo criado com sucesso",
		"Attribute updated successfully":            "Atributo atualizado com sucesso",
		"Attribute deleted successfully":            "Atributo excluído com sucesso",
		"Could not create the attribute (ref %s)":   "Erro ao criar atributo (ref %s)",
		"Could not update the attribute (ref %s)":   "Erro ao atualizar atributo (ref %s)",
		"Could not delete the attribute (ref %s)":   "Erro ao excluir atributo (ref %s)",
		"Could not validate uniqueness (ref %s)":    "Erro ao validar unicidade (ref %s)",
		"Could not save (ref %s)":                   "Erro ao salvar (ref %s)",
		"Could not save the value (ref %s)":         "Erro ao salvar valor (ref %s)",
		"Could not save the script (ref %s)":        "Erro ao salvar script (ref %s)",
		"Could not activate the record (ref %s)":    "Erro ao ativar registro (ref %s)",

		// admin: menus
		"Menu updated successfully":          "Menu atualizado com sucesso",
		"Menu deleted successfully":          "Menu excluído com sucesso",
		"Menu not found":                     "Menu não encontrado",
		"Item created successfully":          "Item criado com sucesso",
		"Item updated successfully":          "Item atualizado com sucesso",
		"Item deleted successfully":          "Item excluído com sucesso",
		"Could not create the menu (ref %s)": "Erro ao criar menu (ref %s)",
		"Could not create the item (ref %s)": "Erro ao criar item (ref %s)",
		"Could not delete the menu":          "Erro ao excluir menu",
		"Could not delete the item":          "Erro ao excluir item",

		// admin: users
		"Could not update the profile (ref %s)":      "Erro ao atualizar perfil (ref %s)",
		"Could not update the sysop flag (ref %s)":   "Erro ao atualizar sysop (ref %s)",
		"Could not update the enabled flag (ref %s)": "Erro ao atualizar enabled (ref %s)",

		// file manager
		"%s used":       "%s usados",
		"%s of %s used": "%s de %s usados",
		"Upload exceeds your storage quota of %s": "O upload excede sua cota de armazenamento de %s",
		"Select a file to upload":                 "Por favor, selecione um arquivo",
		"Invalid file: %s":                        "Arquivo inválido: %s",

		// record search
		"Global search": "Busca global",
		"One query across the text fields of every table.": "Uma consulta sobre os campos de texto de todas as tabelas.",
		"Type and press Enter":                             "Digite e pressione Enter",
		"Search":                                           "Buscar",
		"See all results":                                  "Ver todos os resultados",
		"See all records":                                  "Ver todos os registros",
		"No records match %q":                              "Nenhum registro corresponde a %q",

		// Filo REPL
		"Invalid globals JSON: %s": "Globals JSON inválido: %s",
	})
}
