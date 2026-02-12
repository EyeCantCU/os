import torch
import torch.nn as nn
import torch.nn.functional as F

# Define a simple model
class SimpleModel(nn.Module):
    def __init__(self):
        super(SimpleModel, self).__init__()
        self.fc1 = nn.Linear(10, 50)
        self.fc2 = nn.Linear(50, 1)

    def forward(self, x):
        x = F.relu(self.fc1(x))
        return self.fc2(x)

# Instantiate the model
model = SimpleModel()

# Compile the model (PyTorch 2.0+)
compiled_model = torch.compile(model)

# Dummy input
input_tensor = torch.randn(32, 10)  # batch of 32, 10 features each

# Forward pass
output = compiled_model(input_tensor)
print(output)
