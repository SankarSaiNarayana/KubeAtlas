from langchain_groq import ChatGroq
from langchain_core.messages import HumanMessage
import os

llm = ChatGroq(
    model="llama-3.3-70b-versatile",
    api_key=os.getenv("GROQ_API_KEY"),
    temperature=0.1,
)

message = input("You:good morning")

response = llm.invoke([
    HumanMessage(content=message)
])

print("\nAI:", response.content)