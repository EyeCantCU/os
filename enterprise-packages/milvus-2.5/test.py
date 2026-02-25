#!/usr/bin/env python3
"""
Functional test for Milvus 2.5 standalone instance.
Tests basic vector database operations: create collection, insert data, search, and query.
"""

import sys
import time
from pymilvus import (
    connections,
    utility,
    Collection,
    CollectionSchema,
    FieldSchema,
    DataType,
)

# Configuration
MILVUS_HOST = "localhost"
MILVUS_PORT = "19530"
COLLECTION_NAME = "test_collection"
DIMENSION = 128


def connect_to_milvus():
    """Connect to Milvus server with retry logic."""
    print(f"Connecting to Milvus at {MILVUS_HOST}:{MILVUS_PORT}...")
    max_retries = 5
    for i in range(max_retries):
        try:
            connections.connect(
                alias="default",
                host=MILVUS_HOST,
                port=MILVUS_PORT,
                timeout=10
            )
            print("✓ Successfully connected to Milvus")
            return
        except Exception as e:
            if i < max_retries - 1:
                print(f"Connection attempt {i+1} failed, retrying in 2s...")
                time.sleep(2)
            else:
                print(f"✗ Failed to connect to Milvus after {max_retries} attempts: {e}")
                raise


def check_server_version():
    """Verify server version."""
    version = utility.get_server_version()
    print(f"✓ Milvus server version: {version}")
    if not version.startswith("2.5"):
        print(f"✗ Expected version 2.5.x, got {version}")
        sys.exit(1)


def create_collection():
    """Create a test collection with vector and scalar fields."""
    print(f"\nCreating collection '{COLLECTION_NAME}'...")

    # Drop collection if it already exists
    if utility.has_collection(COLLECTION_NAME):
        print(f"Collection '{COLLECTION_NAME}' already exists, dropping it...")
        utility.drop_collection(COLLECTION_NAME)

    # Define schema
    fields = [
        FieldSchema(name="id", dtype=DataType.INT64, is_primary=True, auto_id=False),
        FieldSchema(name="embeddings", dtype=DataType.FLOAT_VECTOR, dim=DIMENSION),
        FieldSchema(name="text", dtype=DataType.VARCHAR, max_length=512),
        FieldSchema(name="score", dtype=DataType.FLOAT),
    ]

    schema = CollectionSchema(
        fields=fields,
        description="Test collection for Milvus functional testing"
    )

    collection = Collection(
        name=COLLECTION_NAME,
        schema=schema,
        using="default"
    )

    print(f"✓ Created collection with {len(fields)} fields")
    return collection


def insert_test_data(collection):
    """Insert test vectors and metadata into the collection."""
    print("\nInserting test data...")

    # Generate test data
    num_entities = 100
    import random
    random.seed(42)

    ids = list(range(num_entities))
    embeddings = [[random.random() for _ in range(DIMENSION)] for _ in range(num_entities)]
    texts = [f"test_document_{i}" for i in range(num_entities)]
    scores = [random.random() * 100 for _ in range(num_entities)]

    # Insert data
    insert_result = collection.insert([ids, embeddings, texts, scores])

    # Flush to ensure data is persisted
    collection.flush()

    print(f"✓ Inserted {num_entities} entities")
    print(f"✓ Insert IDs count: {len(insert_result.primary_keys)}")

    # Verify entity count
    entity_count = collection.num_entities
    if entity_count != num_entities:
        print(f"✗ Expected {num_entities} entities, got {entity_count}")
        sys.exit(1)

    print(f"✓ Verified entity count: {entity_count}")
    return num_entities


def create_index(collection):
    """Create IVF_FLAT index for vector search."""
    print("\nCreating index...")

    index_params = {
        "metric_type": "L2",
        "index_type": "IVF_FLAT",
        "params": {"nlist": 16}
    }

    collection.create_index(
        field_name="embeddings",
        index_params=index_params
    )

    print("✓ Created IVF_FLAT index on embeddings field")


def load_collection(collection):
    """Load collection into memory for searching."""
    print("\nLoading collection...")
    collection.load()
    print("✓ Collection loaded into memory")


def perform_vector_search(collection):
    """Perform vector similarity search."""
    print("\nPerforming vector search...")

    # Generate query vector
    import random
    random.seed(123)
    query_vector = [[random.random() for _ in range(DIMENSION)]]

    # Search parameters
    search_params = {
        "metric_type": "L2",
        "params": {"nprobe": 8}
    }

    # Perform search
    results = collection.search(
        data=query_vector,
        anns_field="embeddings",
        param=search_params,
        limit=5,
        output_fields=["text", "score"]
    )

    print(f"✓ Search returned {len(results[0])} results")

    # Verify results
    if len(results[0]) != 5:
        print(f"✗ Expected 5 search results, got {len(results[0])}")
        sys.exit(1)

    # Display top result
    top_result = results[0][0]
    print(f"✓ Top result: ID={top_result.id}, distance={top_result.distance:.4f}, "
          f"text='{top_result.entity.get('text')}'")


def perform_query(collection):
    """Query data using expression filter."""
    print("\nPerforming filtered query...")

    # Query entities with score > 50
    results = collection.query(
        expr="score > 50",
        output_fields=["id", "text", "score"],
        limit=10
    )

    print(f"✓ Query returned {len(results)} results")

    # Verify results
    if len(results) == 0:
        print("✗ Expected at least some results with score > 50")
        sys.exit(1)

    # Verify all results match the filter
    for result in results:
        if result["score"] <= 50:
            print(f"✗ Result with score {result['score']} does not match filter 'score > 50'")
            sys.exit(1)

    print(f"✓ All results match filter condition (score > 50)")
    print(f"  Sample result: ID={results[0]['id']}, "
          f"text='{results[0]['text']}', score={results[0]['score']:.2f}")


def cleanup(collection):
    """Clean up test collection."""
    print("\nCleaning up...")
    collection.release()
    utility.drop_collection(COLLECTION_NAME)
    print(f"✓ Dropped collection '{COLLECTION_NAME}'")


def main():
    """Run all tests."""
    print("=" * 60)
    print("Milvus 2.5 Functional Test - PyMilvus Client")
    print("=" * 60)

    try:
        # Connect to Milvus
        connect_to_milvus()
        check_server_version()

        # Create collection and insert data
        collection = create_collection()
        num_entities = insert_test_data(collection)

        # Index and load
        create_index(collection)
        load_collection(collection)

        # Perform searches and queries
        perform_vector_search(collection)
        perform_query(collection)

        # Cleanup
        cleanup(collection)

        print("\n" + "=" * 60)
        print("✓ All tests passed successfully!")
        print("=" * 60)

        connections.disconnect("default")
        sys.exit(0)

    except Exception as e:
        print(f"\n✗ Test failed with error: {e}")
        import traceback
        traceback.print_exc()

        # Try to disconnect
        try:
            connections.disconnect("default")
        except:
            pass

        sys.exit(1)


if __name__ == "__main__":
    main()
