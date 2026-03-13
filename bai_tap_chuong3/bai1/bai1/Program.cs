

using System.Diagnostics;

class Program
{
	static void Main(string[] args)
	{
		// For process pool
		if (args.Length > 0 && args[0] == "worker")
		{
			int start = int.Parse(args[1]);
			int end = int.Parse(args[2]);

			long sum = 0;

			for (int i = start; i <= end; i++)
			{
				sum += i;
			}

			Console.WriteLine(sum);
			return;
		}


		// Main process flow
		int N = 1000000000; // 1 billion
		int workers = 4;

		Console.WriteLine("Running Single Process...");
		RunSingle(N);

		Console.WriteLine("Running Process Pool...");
		RunProcessPool(N, workers);

		Console.WriteLine("Running Thread Pool...");
		RunThreadPool(N, workers);
	}


	/* 
	====== Single Process======
	 */
	static void RunSingle(int N)
	{
		var sw = Stopwatch.StartNew();
		long sum = 0;
		for (int i = 1; i <= N; i++)
		{
			sum += i;
		}
		sw.Stop();
		Console.WriteLine($"Sum: {sum}, Time: {sw.ElapsedMilliseconds} ms");
	}


	/* 
	====== Process Pool======
	 */
	static void RunProcessPool(int N, int workers)
	{
		var sw = Stopwatch.StartNew();

		int chunk = N / workers;

		long sum = 0;

		List<Process> processes = new List<Process>();

		for (int i = 0; i < workers; i++)   // start all worker processes
		{
			int start = i * chunk + 1;
			int end = (i == workers - 1) ? N : (i + 1) * chunk;

			var process = new Process();
			process.StartInfo.FileName = "./bin/Debug/net10.0/bai1";
			process.StartInfo.Arguments = $"worker {start} {end}";
			process.StartInfo.RedirectStandardOutput = true;
			process.StartInfo.UseShellExecute = false;

			process.Start();

			processes.Add(process);
		}

		foreach (var process in processes)  // read output from each worker process
		{
			string output = process.StandardOutput.ReadToEnd();
			process.WaitForExit();

			sum += long.Parse(output.Trim());
		}

		sw.Stop();
		Console.WriteLine($"Sum: {sum}, Time: {sw.ElapsedMilliseconds} ms");
	}

	/* 
	====== Thread Pool======
	 */
	static void RunThreadPool(int N, int workers)
	{
		var sw = Stopwatch.StartNew();

		int chunk = N / workers;

		long sum = 0;

		object locker = new Object();

		Parallel.For(0, workers, new ParallelOptions { MaxDegreeOfParallelism = workers }, i =>
		{
			int start = i * chunk + 1;
			int end = (i == workers - 1) ? N : (i + 1) * chunk;

			long localSum = 0;
			for (int j = start; j <= end; j++)
			{
				localSum += j;
			}

			lock (locker)
			{
				sum += localSum;
			}
		});

		sw.Stop();
		Console.WriteLine($"Sum: {sum}, Time: {sw.ElapsedMilliseconds} ms");
	}

}




