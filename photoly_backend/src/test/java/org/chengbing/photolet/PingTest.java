package org.chengbing.photolet;

import java.io.IOException;
import java.net.InetAddress;
import java.net.UnknownHostException;

/**
 * @Author: Harold澂冰
 * @Date: 2022/7/22 10:58
 */
public class PingTest {

    public static void main(String[] args) throws IOException {
        InetAddress address = InetAddress.getByName("129.146.3.179");
        boolean reachable = address.isReachable(10000);

        System.out.println("Is host reachable? " + reachable);
    }
}
