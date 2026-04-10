import openviking as ov

class OpenVK:
    
    client_list = {}
    
    @classmethod
    def get_client(cls, tenant_id="workspace"):
        if tenant_id in cls.client_list:
            return cls.client_list[tenant_id]

        # Connect to remote services
        try:
            client = ov.OpenViking(path=f"./data/{tenant_id}",)
            client.initialize()
            cls.client_list[tenant_id] = client
            return client
        except Exception as e:
                print(f"Error connecting to OpenViking: {e}")

        return None


    @classmethod
    def close_client(cls, tenant_id=None):
        if tenant_id in cls.client_list:
            client_to_close = cls.client_list[tenant_id]
            client_to_close.close()
            del cls.client_list[tenant_id]

