import openviking as ov

class OpenVK:
    
    client_list = []
    
    @classmethod
    def get_client(cls, tenant_id="workspace"):
        for client in cls.client_list:
            if client.tenant_id == tenant_id:
                return client

        # Connect to remote services
        try:
            client = ov.OpenViking(path=f"./data/{tenant_id}",)
            client.initialize()
            cls.client_list.append(client)
            return client
        except Exception as e:
                print(f"Error connecting to OpenViking: {e}")

        return None


    @classmethod
    def close_client(cls):
        if cls._client is not None:
            cls._client.close()
            cls._client = None

