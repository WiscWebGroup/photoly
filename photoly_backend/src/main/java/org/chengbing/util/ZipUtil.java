package org.chengbing.util;

import org.apache.commons.compress.utils.IOUtils;

import java.io.*;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.zip.ZipEntry;
import java.util.zip.ZipOutputStream;

public class ZipUtil {

    public static void compressZipFile(String sourceDir, String outputFile) throws IOException {
        ZipOutputStream zipFile = new ZipOutputStream(new FileOutputStream(outputFile));
        Path srcPath = Paths.get(sourceDir);
        compressDirectoryToZipFile(srcPath.getParent().toString(), srcPath.getFileName().toString(), zipFile);
        IOUtils.closeQuietly(zipFile);
    }

    private static void compressDirectoryToZipFile(String rootDir, String sourceDir, ZipOutputStream out) throws IOException {
        String dir = Paths.get(rootDir, sourceDir).toString();
        for (File file : new File(dir).listFiles()) {
            if (file.isDirectory()) {
                compressDirectoryToZipFile(rootDir, Paths.get(sourceDir,file.getName()).toString(), out);
            } else {
                ZipEntry entry = new ZipEntry(Paths.get(sourceDir,file.getName()).toString());
                out.putNextEntry(entry);

                FileInputStream in = new FileInputStream(Paths.get(rootDir, sourceDir, file.getName()).toString());
                IOUtils.copy(in, out);
                IOUtils.closeQuietly(in);
            }
        }
    }



    public static void main(String[] args) {
        String testDir = "D:\\Upload";
        String testDestZipName = "D:\\test\\upload.zip";

        try {
            ZipUtil.compressZipFile(testDir, testDestZipName);
        } catch (IOException e) {
            throw new RuntimeException(e);
        }
    }
}
