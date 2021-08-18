class CommandError(Exception):
    def __init__(self, message):
        self.message = message

class InvalidKluctlProjectConfig(Exception):
    def __init__(self, message, config=None):
        self.message = message
        self.config = config
